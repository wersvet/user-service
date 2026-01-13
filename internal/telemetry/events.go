package telemetry

import "time"

const (
	AuditEventType    = "audit_event"
	AuditEventVersion = "v1"
	AuditFriendsKey   = "audit.friends"
)

type Config struct {
	Environment string
	ServiceName string
}

type Envelope struct {
	EventType   string `json:"event_type"`
	Version     string `json:"version"`
	Timestamp   string `json:"timestamp"`
	Environment string `json:"environment"`
	Service     string `json:"service"`
	RequestID   string `json:"request_id"`
	TraceID     string `json:"trace_id"`
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
	WSSessionID string `json:"ws_session_id"`
	Payload     any    `json:"payload"`
}

func NewEnvelope(cfg Config, requestID, userID string, payload any) Envelope {
	return Envelope{
		EventType:   AuditEventType,
		Version:     AuditEventVersion,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Environment: cfg.Environment,
		Service:     cfg.ServiceName,
		RequestID:   requestID,
		TraceID:     "",
		UserID:      userID,
		DeviceID:    "",
		WSSessionID: "",
		Payload:     payload,
	}
}

type FriendRequestSentPayload struct {
	Action    string `json:"action"`
	RequestID string `json:"request_id,omitempty"`
	FromUser  string `json:"from_user_id,omitempty"`
	ToUser    string `json:"to_user_id,omitempty"`
	Status    string `json:"status"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
}

type FriendRequestDecisionPayload struct {
	Action          string `json:"action"`
	FriendRequestID string `json:"friend_request_id,omitempty"`
	ByUser          string `json:"by_user_id,omitempty"`
	Status          string `json:"status"`
	Result          string `json:"result"`
	Error           string `json:"error,omitempty"`
}
