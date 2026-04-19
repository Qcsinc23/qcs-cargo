package services

import (
	"bufio"
	"container/list"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultRedisPort    = "6379"
	defaultRedisTimeout = 2 * time.Second

	// defaultMemoryCacheCap caps the number of entries the in-process
	// MemoryCache will hold before evicting the least-recently used key.
	// Pass 2.5 MED-10: previously the map grew without bound, allowing a
	// hostile or buggy caller to OOM the process by storing distinct keys.
	defaultMemoryCacheCap = 4096
)

// Cache is a lightweight abstraction used by API handlers and middleware.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Ping(ctx context.Context) error
	Backend() string
}

// NewCacheFromEnv returns memory cache by default and redis+memory tiered cache
// when REDIS_URL is configured.
func NewCacheFromEnv() Cache {
	mem := NewMemoryCache()
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		return mem
	}

	redisCache, err := NewRedisCache(redisURL)
	if err != nil {
		log.Printf("[cache] invalid REDIS_URL, using memory cache only: %v", err)
		return mem
	}

	return &tieredCache{
		primary:  redisCache,
		fallback: mem,
		backend:  "redis+memory",
	}
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
	// elem points at this key's node in the LRU list so Get/Set can
	// reorder in O(1) without scanning.
	elem *list.Element
}

// MemoryCache is an in-process cache suitable for single-node deployments.
//
// Pass 2.5 MED-10: bounded LRU. The cache caps the number of entries at
// `cap` (default defaultMemoryCacheCap). Each Set that would push the
// cache past the cap evicts the least-recently used key first. Each Get
// promotes the accessed key to the back of the LRU list. The previous
// implementation grew the map without bound, which let any caller that
// produced unbounded distinct keys (e.g. cache-keyed by IP, query
// string, or per-request token) eventually exhaust process memory.
type MemoryCache struct {
	mu    sync.Mutex
	items map[string]*memoryItem
	lru   *list.List // front = least recently used; back = most recent
	cap   int
}

func NewMemoryCache() *MemoryCache {
	return NewMemoryCacheWithCap(defaultMemoryCacheCap)
}

// NewMemoryCacheWithCap returns a MemoryCache with an explicit capacity.
// A non-positive cap falls back to defaultMemoryCacheCap. Exposed for
// test coverage of the eviction path; production callers should use
// NewMemoryCache.
func NewMemoryCacheWithCap(cap int) *MemoryCache {
	if cap <= 0 {
		cap = defaultMemoryCacheCap
	}
	return &MemoryCache{
		items: make(map[string]*memoryItem),
		lru:   list.New(),
		cap:   cap,
	}
}

func (m *MemoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	now := time.Now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[key]
	if !ok {
		return nil, false, nil
	}

	if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
		m.removeLocked(key, item)
		return nil, false, nil
	}

	// LRU promotion: most recently accessed entry moves to the back so
	// the front of the list always points at the eviction candidate.
	if item.elem != nil {
		m.lru.MoveToBack(item.elem)
	}

	return append([]byte(nil), item.value...), true, nil
}

func (m *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	item := &memoryItem{
		value: append([]byte(nil), value...),
	}
	if ttl > 0 {
		item.expiresAt = time.Now().UTC().Add(ttl)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.items[key]; ok {
		existing.value = item.value
		existing.expiresAt = item.expiresAt
		if existing.elem != nil {
			m.lru.MoveToBack(existing.elem)
		}
		return nil
	}

	// Evict until there is room for the new entry. The eviction loop
	// tolerates corrupt list entries by falling through and shrinking
	// the map directly if the LRU list is empty for any reason.
	for len(m.items) >= m.cap {
		oldest := m.lru.Front()
		if oldest == nil {
			break
		}
		oldKey, _ := oldest.Value.(string)
		if oldItem, ok := m.items[oldKey]; ok {
			m.removeLocked(oldKey, oldItem)
		} else {
			m.lru.Remove(oldest)
		}
	}

	item.elem = m.lru.PushBack(key)
	m.items[key] = item
	return nil
}

func (m *MemoryCache) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item, ok := m.items[key]; ok {
		m.removeLocked(key, item)
	}
	return nil
}

// removeLocked drops a key from both the map and the LRU list. Caller
// must hold m.mu.
func (m *MemoryCache) removeLocked(key string, item *memoryItem) {
	if item != nil && item.elem != nil {
		m.lru.Remove(item.elem)
	}
	delete(m.items, key)
}

// Len reports the current number of cached entries. Primarily used by
// tests to assert the bounded-cache invariant.
func (m *MemoryCache) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

// Cap reports the configured maximum number of entries.
func (m *MemoryCache) Cap() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cap
}

func (m *MemoryCache) Ping(_ context.Context) error {
	return nil
}

func (m *MemoryCache) Backend() string {
	return "memory"
}

type tieredCache struct {
	primary  Cache
	fallback Cache
	backend  string
}

func (t *tieredCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	var firstErr error

	if t.primary != nil {
		value, ok, err := t.primary.Get(ctx, key)
		if err == nil && ok {
			return value, true, nil
		}
		if err != nil {
			firstErr = err
		}
	}

	if t.fallback != nil {
		value, ok, err := t.fallback.Get(ctx, key)
		if err == nil && ok {
			if t.primary != nil {
				_ = t.primary.Set(ctx, key, value, 30*time.Second)
			}
			return value, true, nil
		}
		if firstErr == nil && err != nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		return nil, false, firstErr
	}
	return nil, false, nil
}

func (t *tieredCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var errs []string
	writes := 0

	if t.primary != nil {
		writes++
		if err := t.primary.Set(ctx, key, value, ttl); err != nil {
			errs = append(errs, "primary: "+err.Error())
		}
	}
	if t.fallback != nil {
		writes++
		if err := t.fallback.Set(ctx, key, value, ttl); err != nil {
			errs = append(errs, "fallback: "+err.Error())
		}
	}

	if writes > 0 && len(errs) == writes {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (t *tieredCache) Delete(ctx context.Context, key string) error {
	var errs []string
	deletes := 0

	if t.primary != nil {
		deletes++
		if err := t.primary.Delete(ctx, key); err != nil {
			errs = append(errs, "primary: "+err.Error())
		}
	}
	if t.fallback != nil {
		deletes++
		if err := t.fallback.Delete(ctx, key); err != nil {
			errs = append(errs, "fallback: "+err.Error())
		}
	}

	if deletes > 0 && len(errs) == deletes {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (t *tieredCache) Ping(ctx context.Context) error {
	if t.primary != nil {
		return t.primary.Ping(ctx)
	}
	if t.fallback != nil {
		return t.fallback.Ping(ctx)
	}
	return nil
}

func (t *tieredCache) Backend() string {
	return t.backend
}

// RedisCache is a minimal RESP client for cache GET/SET/PING.
// It avoids external dependencies while supporting REDIS_URL based wiring.
type RedisCache struct {
	addr       string
	serverName string
	useTLS     bool
	username   string
	password   string
	db         int
	timeout    time.Duration
}

func NewRedisCache(rawURL string) (*RedisCache, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return nil, fmt.Errorf("unsupported redis scheme: %s", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("redis host is required")
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = defaultRedisPort
	}

	db := 0
	if rawDB := strings.TrimSpace(strings.TrimPrefix(u.Path, "/")); rawDB != "" {
		parsed, parseErr := strconv.Atoi(rawDB)
		if parseErr != nil || parsed < 0 {
			return nil, fmt.Errorf("invalid redis db: %q", rawDB)
		}
		db = parsed
	}

	password, _ := u.User.Password()

	return &RedisCache{
		addr:       net.JoinHostPort(host, port),
		serverName: host,
		useTLS:     u.Scheme == "rediss",
		username:   u.User.Username(),
		password:   password,
		db:         db,
		timeout:    defaultRedisTimeout,
	}, nil
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	conn, err := r.connect(ctx)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()

	resp, err := redisRoundTrip(conn, "GET", key)
	if err != nil {
		return nil, false, err
	}
	if resp.kind != '$' {
		return nil, false, fmt.Errorf("unexpected redis GET response kind: %q", string(resp.kind))
	}
	if resp.nilBulk {
		return nil, false, nil
	}
	return append([]byte(nil), resp.bulk...), true, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	conn, err := r.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if ttl > 0 {
		seconds := int64(ttl / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		_, err = redisRoundTrip(conn, "SET", key, string(value), "EX", strconv.FormatInt(seconds, 10))
		return err
	}

	_, err = redisRoundTrip(conn, "SET", key, string(value))
	return err
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	conn, err := r.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = redisRoundTrip(conn, "DEL", key)
	return err
}

func (r *RedisCache) Ping(ctx context.Context) error {
	conn, err := r.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := redisRoundTrip(conn, "PING")
	if err != nil {
		return err
	}
	if resp.kind != '+' {
		return fmt.Errorf("unexpected redis ping response kind: %q", string(resp.kind))
	}
	return nil
}

func (r *RedisCache) Backend() string {
	return "redis"
}

func (r *RedisCache) connect(ctx context.Context) (net.Conn, error) {
	timeout := r.timeout
	if timeout <= 0 {
		timeout = defaultRedisTimeout
	}

	var conn net.Conn
	var err error

	dialer := net.Dialer{Timeout: timeout}
	if r.useTLS {
		conn, err = tls.DialWithDialer(&dialer, "tcp", r.addr, &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: r.serverName,
		})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", r.addr)
	}
	if err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}

	if r.password != "" {
		if r.username != "" {
			if _, err := redisRoundTrip(conn, "AUTH", r.username, r.password); err != nil {
				_ = conn.Close()
				return nil, err
			}
		} else {
			if _, err := redisRoundTrip(conn, "AUTH", r.password); err != nil {
				_ = conn.Close()
				return nil, err
			}
		}
	}

	if r.db > 0 {
		if _, err := redisRoundTrip(conn, "SELECT", strconv.Itoa(r.db)); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

type redisResponse struct {
	kind    byte
	text    string
	bulk    []byte
	nilBulk bool
}

func redisRoundTrip(conn net.Conn, args ...string) (redisResponse, error) {
	if err := writeRESPCommand(conn, args...); err != nil {
		return redisResponse{}, err
	}

	resp, err := readRESP(conn)
	if err != nil {
		return redisResponse{}, err
	}
	if resp.kind == '-' {
		return redisResponse{}, errors.New(resp.text)
	}
	return resp, nil
}

func writeRESPCommand(w io.Writer, args ...string) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	return nil
}

func readRESP(r io.Reader) (redisResponse, error) {
	reader := bufio.NewReader(r)

	kind, err := reader.ReadByte()
	if err != nil {
		return redisResponse{}, err
	}

	switch kind {
	case '+', '-', ':':
		line, err := readRESPLine(reader)
		if err != nil {
			return redisResponse{}, err
		}
		return redisResponse{kind: kind, text: line}, nil
	case '$':
		line, err := readRESPLine(reader)
		if err != nil {
			return redisResponse{}, err
		}
		size, err := strconv.Atoi(line)
		if err != nil {
			return redisResponse{}, err
		}
		if size == -1 {
			return redisResponse{kind: kind, nilBulk: true}, nil
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return redisResponse{}, err
		}
		if _, err := io.ReadFull(reader, make([]byte, 2)); err != nil {
			return redisResponse{}, err
		}
		return redisResponse{kind: kind, bulk: buf}, nil
	default:
		return redisResponse{}, fmt.Errorf("unsupported redis response kind: %q", string(kind))
	}
}

func readRESPLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}
