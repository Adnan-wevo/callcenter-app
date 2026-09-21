// Package pbxworker is the HTTP gateway to the pbx-worker service that
// serves historical call-center reports.
//
// ASSUMPTION — every struct/field below is a guess at a reasonable shape
// for queue reporting data. None of it is confirmed against the real
// pbx-worker v2 (PHP, /api/reports.php) response bodies. Confirm and adjust
// once the reference code lands.
package pbxworker

import "time"

type QueueName struct {
	ID        string `json:"id"`
	Extension string `json:"extension"`
	Name      string `json:"name"`
}

type AgentName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AnsweredCall struct {
	ID              string    `json:"id"`
	QueueID         string    `json:"queue_id"`
	QueueName       string    `json:"queue_name"`
	AgentID         string    `json:"agent_id"`
	AgentName       string    `json:"agent_name"`
	CallerID        string    `json:"caller_id"`
	EnteredAt       time.Time `json:"entered_at"`
	AnsweredAt      time.Time `json:"answered_at"`
	EndedAt         time.Time `json:"ended_at"`
	WaitSeconds     int       `json:"wait_seconds"`
	TalkSeconds     int       `json:"talk_seconds"`
	RecordingURL    string    `json:"recording_url,omitempty"` // TODO: confirm whether pbx-worker returns this directly or if it must be looked up from qstats.recordings
}

type UnansweredCall struct {
	ID          string    `json:"id"`
	QueueID     string    `json:"queue_id"`
	QueueName   string    `json:"queue_name"`
	CallerID    string    `json:"caller_id"`
	EnteredAt   time.Time `json:"entered_at"`
	AbandonedAt time.Time `json:"abandoned_at"`
	WaitSeconds int       `json:"wait_seconds"`
	Reason      string    `json:"reason"` // ASSUMPTION enum: ABANDONED, TIMEOUT, ...
}

type AgentEvent struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	AgentName string    `json:"agent_name"`
	QueueID   string    `json:"queue_id"`
	EventType string    `json:"event_type"`
	EventTime time.Time `json:"event_time"`
}

type CallSummary struct {
	ID         string    `json:"id"`
	QueueID    string    `json:"queue_id"`
	AgentID    string    `json:"agent_id,omitempty"`
	CallerID   string    `json:"caller_id"`
	Status     string    `json:"status"` // ASSUMPTION enum: ANSWERED, ABANDONED, TIMEOUT
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
}

type CallDetail struct {
	CallSummary
	WaitSeconds  int          `json:"wait_seconds"`
	TalkSeconds  int          `json:"talk_seconds"`
	RecordingURL string       `json:"recording_url,omitempty"`
	Events       []AgentEvent `json:"events,omitempty"`
}

// Pagination mirrors what we assume pbx-worker returns for list endpoints.
type Pagination struct {
	CurrentPage int `json:"current_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
	LastPage    int `json:"last_page"`
}

// ListParams covers the filter/pagination query params assumed common to
// the answered-calls, unanswered-calls and agent-events actions.
type ListParams struct {
	QueueID string
	AgentID string
	From    *time.Time
	To      *time.Time
	Page    int
	PerPage int
}

// CallSearchParams covers the call-search action's assumed filters.
type CallSearchParams struct {
	ListParams
	CallerID string
	Status   string
}
