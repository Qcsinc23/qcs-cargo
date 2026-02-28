package api

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/google/uuid"
)

// recordActivity writes an audit event to admin_activity. It is best-effort and must not break request flows.
func recordActivity(ctx context.Context, actorID, action, entityType, entityID, details string) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return
	}
	arg := gen.CreateAdminActivityParams{
		ID:         uuid.New().String(),
		ActorID:    actorID,
		Action:     strings.TrimSpace(action),
		EntityType: strings.TrimSpace(entityType),
		EntityID:   sql.NullString{},
		Details:    sql.NullString{},
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(entityID) != "" {
		arg.EntityID = sql.NullString{String: strings.TrimSpace(entityID), Valid: true}
	}
	if strings.TrimSpace(details) != "" {
		arg.Details = sql.NullString{String: strings.TrimSpace(details), Valid: true}
	}
	if err := db.Queries().CreateAdminActivity(ctx, arg); err != nil {
		log.Printf("activity log write failed: actor_id=%s action=%s entity_type=%s entity_id=%s err=%v", actorID, action, entityType, entityID, err)
	}
}

