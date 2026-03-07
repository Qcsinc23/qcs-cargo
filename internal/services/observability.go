package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
)

const (
	observabilityQueueSize = 512
	observabilityDBTimeout = 2 * time.Second
)

// ObservabilityEvent is the runtime event payload persisted for analytics/perf/error insights.
type ObservabilityEvent struct {
	Category   string
	EventName  string
	UserID     string
	RequestID  string
	Path       string
	Method     string
	StatusCode *int
	DurationMS *float64
	Value      *float64
	Metadata   map[string]any
	CreatedAt  time.Time
}

type ObservabilityService struct {
	queue         chan ObservabilityEvent
	sentryEnabled bool
	disabled      bool
}

var (
	observabilityOnce sync.Once
	observabilitySvc  *ObservabilityService
)

// ResetObservabilityForTest clears the process-wide singleton between tests.
func ResetObservabilityForTest() {
	if observabilitySvc != nil && observabilitySvc.queue != nil && !observabilitySvc.disabled {
		close(observabilitySvc.queue)
	}
	observabilitySvc = nil
	observabilityOnce = sync.Once{}
}

// Observability returns a process-wide singleton service.
func Observability() *ObservabilityService {
	observabilityOnce.Do(func() {
		observabilitySvc = newObservabilityService()
	})
	return observabilitySvc
}

func newObservabilityService() *ObservabilityService {
	svc := &ObservabilityService{
		queue: make(chan ObservabilityEvent, observabilityQueueSize),
	}
	if envFlagEnabled("QCS_OBSERVABILITY_DISABLED") {
		svc.disabled = true
		return svc
	}
	svc.initSentry()

	go svc.runWorker()
	return svc
}

func (s *ObservabilityService) initSentry() {
	dsn := strings.TrimSpace(os.Getenv("SENTRY_DSN"))
	if dsn == "" {
		return
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		AttachStacktrace: true,
		Environment:      strings.TrimSpace(os.Getenv("APP_ENV")),
	}); err != nil {
		log.Printf("[observability] sentry init failed: %v", err)
		return
	}
	s.sentryEnabled = true
}

func (s *ObservabilityService) runWorker() {
	for event := range s.queue {
		s.persistToDB(event)
	}
}

// Record enqueues an observability event for asynchronous best-effort persistence.
func (s *ObservabilityService) Record(event ObservabilityEvent) {
	if s.disabled {
		return
	}
	if strings.TrimSpace(event.Category) == "" {
		return
	}
	if strings.TrimSpace(event.EventName) == "" {
		event.EventName = event.Category
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	select {
	case s.queue <- event:
	default:
		log.Printf("[observability] queue full, dropping event category=%s name=%s", event.Category, event.EventName)
	}
}

// RecordError captures an internal error as an observability event and sends to Sentry when enabled.
func (s *ObservabilityService) RecordError(err error, event ObservabilityEvent) {
	if s.disabled {
		return
	}
	if strings.TrimSpace(event.Category) == "" {
		event.Category = "error"
	}
	if strings.TrimSpace(event.EventName) == "" {
		event.EventName = "server.error"
	}
	s.Record(event)

	if err != nil {
		s.captureSentry(err, event)
	}
}

func (s *ObservabilityService) captureSentry(err error, event ObservabilityEvent) {
	if !s.sentryEnabled {
		return
	}

	go sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		if event.Category != "" {
			scope.SetTag("category", event.Category)
		}
		if event.EventName != "" {
			scope.SetTag("event_name", event.EventName)
		}
		if event.RequestID != "" {
			scope.SetTag("request_id", event.RequestID)
		}
		if event.Path != "" {
			scope.SetTag("path", event.Path)
		}
		if event.Method != "" {
			scope.SetTag("method", event.Method)
		}
		if event.UserID != "" {
			scope.SetUser(sentry.User{ID: event.UserID})
		}
		if event.StatusCode != nil {
			scope.SetExtra("status_code", *event.StatusCode)
		}
		if event.DurationMS != nil {
			scope.SetExtra("duration_ms", *event.DurationMS)
		}
		for key, value := range event.Metadata {
			scope.SetExtra(key, value)
		}
		sentry.CaptureException(err)
	})
}

func (s *ObservabilityService) persistToDB(event ObservabilityEvent) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[observability] panic while persisting event: %v", r)
		}
	}()

	params := gen.CreateObservabilityEventParams{
		ID:           uuid.NewString(),
		Category:     strings.TrimSpace(event.Category),
		EventName:    strings.TrimSpace(event.EventName),
		UserID:       toNullString(event.UserID),
		RequestID:    toNullString(event.RequestID),
		Path:         toNullString(event.Path),
		Method:       toNullString(event.Method),
		StatusCode:   toNullInt64Pointer(event.StatusCode),
		DurationMs:   toNullFloat64Pointer(event.DurationMS),
		Value:        toNullFloat64Pointer(event.Value),
		MetadataJson: toNullJSONString(event.Metadata),
		CreatedAt:    event.CreatedAt.UTC().Format(time.RFC3339Nano),
	}

	ctx, cancel := context.WithTimeout(context.Background(), observabilityDBTimeout)
	defer cancel()

	if err := db.Queries().CreateObservabilityEvent(ctx, params); err != nil {
		log.Printf("[observability] failed to persist event category=%s name=%s: %v", event.Category, event.EventName, err)
	}
}

func toNullString(value string) sql.NullString {
	v := strings.TrimSpace(value)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{
		String: v,
		Valid:  true,
	}
}

func toNullInt64Pointer(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{
		Int64: int64(*value),
		Valid: true,
	}
}

func toNullFloat64Pointer(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{
		Float64: *value,
		Valid:   true,
	}
}

func toNullJSONString(metadata map[string]any) sql.NullString {
	if len(metadata) == 0 {
		return sql.NullString{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		log.Printf("[observability] failed to encode metadata: %v", err)
		return sql.NullString{}
	}
	return sql.NullString{
		String: string(raw),
		Valid:  true,
	}
}

func envFlagEnabled(name string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
