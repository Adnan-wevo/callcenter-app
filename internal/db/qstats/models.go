// Package qstats provides direct read access to the "qstats" MySQL database
// (separate from the main CRM database).
//
// ASSUMPTION — none of the table/column names or types below are confirmed.
// They are a reasonable guess at a queue-reporting schema based on the table
// names given in the spec (queue_stats, queue_stats_mv, queue_agents,
// queue_events, queue_names, recordings). CONFIRM every struct in this file
// against the real schema once the Laravel reference code/migrations land,
// then delete this notice.
package qstats

import (
	"database/sql"
	"time"
)

// QueueName maps to the queue_names table.
type QueueName struct {
	ID        int64     `db:"id" json:"id"`
	Extension string    `db:"extension" json:"extension"` // ASSUMPTION: queue's PBX extension/number
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// QueueAgent maps to the queue_agents table.
type QueueAgent struct {
	ID        int64     `db:"id" json:"id"`
	AgentID   string    `db:"agent_id" json:"agent_id"` // ASSUMPTION: PBX agent/extension identifier, not the CRM user id
	Name      string    `db:"name" json:"name"`
	QueueID   int64     `db:"queue_id" json:"queue_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// QueueStat maps to the queue_stats table — assumed to be one row per call
// event within a queue.
type QueueStat struct {
	ID            int64          `db:"id" json:"id"`
	QueueID       int64          `db:"queue_id" json:"queue_id"`
	AgentID       sql.NullString `db:"agent_id" json:"agent_id,omitempty"` // null if unanswered/abandoned
	CallID        string         `db:"call_id" json:"call_id"`
	EventType     string         `db:"event_type" json:"event_type"` // ASSUMPTION enum: ANSWERED, ABANDONED, TIMEOUT, ...
	WaitSeconds   int            `db:"wait_seconds" json:"wait_seconds"`
	TalkSeconds   int            `db:"talk_seconds" json:"talk_seconds"`
	StartedAt     time.Time      `db:"started_at" json:"started_at"`
	EndedAt       sql.NullTime   `db:"ended_at" json:"ended_at,omitempty"`
}

// QueueStatMV maps to queue_stats_mv — assumed to be a materialized/rollup
// view aggregated per queue per day.
type QueueStatMV struct {
	QueueID          int64     `db:"queue_id" json:"queue_id"`
	StatDate         time.Time `db:"stat_date" json:"stat_date"`
	TotalCalls       int       `db:"total_calls" json:"total_calls"`
	AnsweredCalls    int       `db:"answered_calls" json:"answered_calls"`
	AbandonedCalls   int       `db:"abandoned_calls" json:"abandoned_calls"`
	AvgWaitSeconds   float64   `db:"avg_wait_seconds" json:"avg_wait_seconds"`
	AvgTalkSeconds   float64   `db:"avg_talk_seconds" json:"avg_talk_seconds"`
}

// QueueEvent maps to queue_events — assumed to be agent state-change events
// (login/logout/pause/etc), distinct from call events in queue_stats.
type QueueEvent struct {
	ID        int64     `db:"id" json:"id"`
	QueueID   int64     `db:"queue_id" json:"queue_id"`
	AgentID   string    `db:"agent_id" json:"agent_id"`
	EventType string    `db:"event_type" json:"event_type"` // ASSUMPTION enum: LOGIN, LOGOUT, PAUSE, UNPAUSE, JOIN, LEAVE
	EventTime time.Time `db:"event_time" json:"event_time"`
	Metadata  sql.NullString `db:"metadata" json:"metadata,omitempty"` // ASSUMPTION: free-form JSON string, e.g. pause reason
}

// Recording maps to the recordings table.
type Recording struct {
	ID         int64     `db:"id" json:"id"`
	CallID     string    `db:"call_id" json:"call_id"`
	QueueID    int64     `db:"queue_id" json:"queue_id"`
	AgentID    sql.NullString `db:"agent_id" json:"agent_id,omitempty"`
	FilePath   string    `db:"file_path" json:"file_path"` // ASSUMPTION: path/URL to the recording file on disk or storage
	DurationSeconds int  `db:"duration_seconds" json:"duration_seconds"`
	RecordedAt time.Time `db:"recorded_at" json:"recorded_at"`
}
