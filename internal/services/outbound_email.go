package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
)

// Phase 3.2 (INC-001 part B): durable outbound email queue.
//
// Callers enqueue an email by template name + recipient + structured
// payload. The worker (internal/jobs/outbound_email.go) drains pending
// rows, re-renders via the template registry below, and dispatches to
// the actual provider sender. Permanent failures (after the configured
// attempt budget) are marked status='failed' for ops review.
//
// Templates are registered at package init by services that already own
// the rendering logic (see RegisterEmailTemplate in services.email or
// added in this file). This keeps the queue layer template-agnostic and
// avoids an import cycle.

// EmailTemplate is the well-known string identifier used in
// outbound_emails.template. Keep this list in lockstep with the
// dispatch table populated via RegisterEmailTemplate.
type EmailTemplate string

const (
	TemplateStorageWarning5d   EmailTemplate = "storage_warning_5d"
	TemplateStorageWarning1d   EmailTemplate = "storage_warning_1d"
	TemplateStorageFinalNotice EmailTemplate = "storage_final_notice"
	TemplateStorageFeeCharged  EmailTemplate = "storage_fee_charged"
	TemplateShipRequestPaid    EmailTemplate = "ship_request_paid"
)

// templateSender renders + sends a queued email from its JSON payload.
// Implementations live in services/email.go.
type templateSender func(ctx context.Context, recipient string, payload json.RawMessage) error

var (
	templateRegistryMu sync.RWMutex
	templateRegistry   = map[EmailTemplate]templateSender{}
)

// RegisterEmailTemplate wires a template name to its renderer/sender.
// Call from package init in the package that owns the template.
func RegisterEmailTemplate(name EmailTemplate, fn templateSender) {
	templateRegistryMu.Lock()
	templateRegistry[name] = fn
	templateRegistryMu.Unlock()
}

// LookupEmailTemplate returns the registered sender, if any.
func LookupEmailTemplate(name EmailTemplate) (templateSender, bool) {
	templateRegistryMu.RLock()
	fn, ok := templateRegistry[name]
	templateRegistryMu.RUnlock()
	return fn, ok
}

// RegisteredEmailTemplates returns the list of currently registered
// template names; used by tests and the worker to validate input.
func RegisteredEmailTemplates() []EmailTemplate {
	templateRegistryMu.RLock()
	defer templateRegistryMu.RUnlock()
	names := make([]EmailTemplate, 0, len(templateRegistry))
	for name := range templateRegistry {
		names = append(names, name)
	}
	return names
}

// EnqueueEmail inserts a pending outbound_emails row. The send happens
// asynchronously via the worker; this call returns once the row is
// durably persisted.
//
// recipient must be a non-empty email address. payload may be nil for
// templates that have no parameters; otherwise it is marshaled to JSON
// and stored verbatim.
func EnqueueEmail(ctx context.Context, template EmailTemplate, recipient string, payload any) error {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return errors.New("EnqueueEmail: recipient is empty")
	}
	if _, ok := LookupEmailTemplate(template); !ok {
		return fmt.Errorf("EnqueueEmail: unknown template %q", template)
	}
	var body []byte
	var err error
	if payload == nil {
		body = []byte("null")
	} else {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("EnqueueEmail: marshal payload: %w", err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return db.Queries().EnqueueOutboundEmail(ctx, gen.EnqueueOutboundEmailParams{
		ID:          "oe_" + uuid.New().String(),
		Template:    string(template),
		Recipient:   recipient,
		PayloadJson: string(body),
		ScheduledAt: now,
		CreatedAt:   now,
	})
}
