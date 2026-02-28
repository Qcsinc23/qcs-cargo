package services

import (
	"bufio"
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
}

// MemoryCache is an in-process cache suitable for single-node deployments.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]memoryItem),
	}
}

func (m *MemoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	now := time.Now().UTC()

	m.mu.RLock()
	item, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}

	if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
		m.mu.Lock()
		delete(m.items, key)
		m.mu.Unlock()
		return nil, false, nil
	}

	return append([]byte(nil), item.value...), true, nil
}

func (m *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	item := memoryItem{
		value: append([]byte(nil), value...),
	}
	if ttl > 0 {
		item.expiresAt = time.Now().UTC().Add(ttl)
	}

	m.mu.Lock()
	m.items[key] = item
	m.mu.Unlock()
	return nil
}

func (m *MemoryCache) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.items, key)
	m.mu.Unlock()
	return nil
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
