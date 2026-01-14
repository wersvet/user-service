package telemetry

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"user-service/internal/rabbitmq"
)

const AuditRoutingKey = "user-service.audit"

const auditSchemaVersion = 1

// Envelope matches the log-collector audit_log schema.
type Envelope struct {
	SchemaVersion int          `json:"schema_version"`
	EventID       string       `json:"event_id"`
	EventType     string       `json:"event_type"`
	OccurredAt    string       `json:"occurred_at"`
	Service       string       `json:"service"`
	Environment   string       `json:"environment"`
	RequestID     string       `json:"request_id"`
	UserID        *int64       `json:"user_id,omitempty"`
	Payload       AuditPayload `json:"payload"`
}

// AuditPayload is the payload for audit_log events.
type AuditPayload struct {
	Level string `json:"level"`
	Text  string `json:"text"`
}

type AuditEmitter struct {
	publisher   rabbitmq.Publisher
	service     string
	environment string
}

func NewAuditEmitter(publisher rabbitmq.Publisher, service, environment string) *AuditEmitter {
	return &AuditEmitter{publisher: publisher, service: service, environment: environment}
}

func (e *AuditEmitter) EmitAudit(ctx context.Context, level, text, requestID string, userID *int64) {
	if e == nil || e.publisher == nil {
		return
	}

	envelope := Envelope{
		SchemaVersion: auditSchemaVersion,
		EventID:       uuid.NewString(),
		EventType:     "audit_log",
		OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
		Service:       e.service,
		Environment:   e.environment,
		RequestID:     requestID,
		UserID:        userID,
		Payload: AuditPayload{
			Level: level,
			Text:  text,
		},
	}

	if err := e.publisher.Publish(ctx, AuditRoutingKey, envelope); err != nil {
		log.Printf("warning: failed to publish audit log: %v", err)
	}
}
