package services

import (
	"context"
	"database/sql"
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

type EmailTemplate string

const (
	TemplateStorageWarning5d   EmailTemplate = "storage_warning_5d"
	TemplateStorageWarning1d   EmailTemplate = "storage_warning_1d"
	TemplateStorageFinalNotice EmailTemplate = "storage_final_notice"
	TemplateStorageFeeCharged  EmailTemplate = "storage_fee_charged"
	TemplateShipRequestPaid    EmailTemplate = "ship_request_paid"
	TemplateMFAChallengeCode   EmailTemplate = "mfa_challenge_code"
)

// Pass 3 CRIT-04: senders receive idempotencyKey (the outbound_emails.id)
// and forward it to Resend as the Idempotency-Key HTTP header.
type templateSender func(ctx context.Context, recipient string, payload json.RawMessage, idempotencyKey string) error

var (
	templateRegistryMu sync.RWMutex
	templateRegistry   = map[EmailTemplate]templateSender{}
)

func RegisterEmailTemplate(name EmailTemplate, fn templateSender) {
	templateRegistryMu.Lock()
	templateRegistry[name] = fn
	templateRegistryMu.Unlock()
}

func LookupEmailTemplate(name EmailTemplate) (templateSender, bool) {
	templateRegistryMu.RLock()
	fn, ok := templateRegistry[name]
	templateRegistryMu.RUnlock()
	return fn, ok
}

func RegisteredEmailTemplates() []EmailTemplate {
	templateRegistryMu.RLock()
	defer templateRegistryMu.RUnlock()
	names := make([]EmailTemplate, 0, len(templateRegistry))
	for name := range templateRegistry {
		names = append(names, name)
	}
	return names
}

func EnqueueEmail(ctx context.Context, template EmailTemplate, recipient string, payload any) error {
	return enqueueEmailQ(ctx, db.Queries(), template, recipient, payload)
}

// Pass 3 CRIT-02 fix: tx-scoped enqueue so the outbound_emails INSERT shares
// the atomic boundary of the surrounding state mutation.
func EnqueueEmailTx(ctx context.Context, tx *sql.Tx, template EmailTemplate, recipient string, payload any) error {
	if tx == nil {
		return errors.New("EnqueueEmailTx: tx is nil")
	}
	return enqueueEmailQ(ctx, db.Queries().WithTx(tx), template, recipient, payload)
}

func enqueueEmailQ(ctx context.Context, q *gen.Queries, template EmailTemplate, recipient string, payload any) error {
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
	return q.EnqueueOutboundEmail(ctx, gen.EnqueueOutboundEmailParams{
		ID:          "oe_" + uuid.New().String(),
		Template:    string(template),
		Recipient:   recipient,
		PayloadJson: string(body),
		ScheduledAt: now,
		CreatedAt:   now,
	})
}
