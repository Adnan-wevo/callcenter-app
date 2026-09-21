// Package qstats provides direct read access to the "qstats" MySQL database
// (separate from the main CRM database).
//
// Schema below is CONFIRMED against the real Laravel Eloquent models
// (Modules/CallCenter/app/Models/{QueueStat,QueueStatMv,QueueAgent,
// QueueEvent,QueueName,Recording}.php) — table names, primary keys, and
// $fillable columns are taken directly from those files. qstats has NO
// local migrations anywhere in the Laravel app; it is an externally-owned
// database that lives on the PBX server itself (see config/tenancy.php),
// so any column not listed below (the Eloquent models declare no
// $fillable for queue_stats/queue_stats_mv, so their full column lists
// are still unconfirmed) should be verified against a live
// `SHOW CREATE TABLE` before use, not guessed.
//
// IMPORTANT — architecture: the real Laravel reporting flow does NOT read
// these tables directly for report data. All report queries go through
// internal/gateway/pbxworker (HTTP to pbx-worker's /api/reports.php,
// mirroring Modules/CallCenter/app/Services/Pbx/PbxReportGateway.php).
// Direct qstats access in the real app is limited to health-check row
// counts (Modules/Monitoring/app/Services/HealthChecks/
// {CallCenterHealthCheck,MysqlHealthCheck}.php). Whether this Go service
// should keep reading qstats directly for queues/agents, or switch to the
// pbx-worker gateway like Laravel does, is an open decision — not resolved
// by this fix, see README.
package qstats

import (
	"database/sql"
	"time"
)

// QueueName maps to the qname table (NOT "queue_names"). PK is queue_id,
// the queue's display name is the "queue" column (NOT "name"). No
// timestamps ($timestamps = false).
type QueueName struct {
	QueueID int64  `db:"queue_id" json:"queue_id"`
	Queue   string `db:"queue" json:"queue"`
}

// QueueAgent maps to the qagent table (NOT "queue_agents"). PK is
// agent_id, the agent's display name is the "agent" column (NOT "name").
// No timestamps, and no queue-linkage column — agents are not tied to a
// single queue at this table's row level.
type QueueAgent struct {
	AgentID int64  `db:"agent_id" json:"agent_id"`
	Agent   string `db:"agent" json:"agent"`
}

// QueueEvent maps to the qevent table (NOT "queue_events"). This is a
// small lookup/dimension table (PK event_id, label column "event"), NOT a
// per-occurrence agent-state-change log as an earlier version of this file
// assumed. No timestamps.
type QueueEvent struct {
	EventID int64  `db:"event_id" json:"event_id"`
	Event   string `db:"event" json:"event"`
}

// QueueStat maps to the queue_stats table. PK is queue_stats_id (NOT
// "id"). Only the columns confirmed via QueueStat.php's belongsTo
// relations are modeled here — the model declares no $fillable, so the
// full column list is unconfirmed. FK column names are exactly as declared
// in those relations, which are unusual: "qname" is the FK column on
// queue_stats pointing at qname.queue_id (same name as the qname table
// itself), likewise "qagent" -> qagent.agent_id and "qevent" ->
// qevent.event_id. "uniqueid" joins to recordings.uniqueid and is also the
// Asterisk call unique-id.
type QueueStat struct {
	QueueStatsID int64          `db:"queue_stats_id" json:"queue_stats_id"`
	Datetime     time.Time      `db:"datetime" json:"datetime"`
	QName        int64          `db:"qname" json:"qname"`                 // FK -> qname.queue_id
	QAgent       sql.NullInt64  `db:"qagent" json:"qagent,omitempty"`     // FK -> qagent.agent_id, null if unanswered
	QEvent       sql.NullInt64  `db:"qevent" json:"qevent,omitempty"`     // FK -> qevent.event_id
	UniqueID     sql.NullString `db:"uniqueid" json:"uniqueid,omitempty"` // FK -> recordings.uniqueid
}

// QueueStatMV maps to the queue_stats_mv table. Only the three datetime
// columns declared by QueueStatMv.php's casts are confirmed. Their
// presence (connect time + end time per row) indicates call-lifecycle
// granularity, NOT a daily aggregate rollup — do not reintroduce
// total_calls/avg_wait_seconds-style fields here without confirming them
// against the real schema first.
type QueueStatMV struct {
	Datetime        time.Time `db:"datetime" json:"datetime"`
	DatetimeConnect time.Time `db:"datetimeconnect" json:"datetimeconnect"`
	DatetimeEnd     time.Time `db:"datetimeend" json:"datetimeend"`
}

// Recording maps to the recordings table. PK is "uniqueid" — a string,
// NOT an auto-increment int — matching Recording.php's
// `$incrementing = false; $keyType = 'string'`. It is the Asterisk call
// unique-id and the join key back to queue_stats.uniqueid. No timestamps.
type Recording struct {
	UniqueID string `db:"uniqueid" json:"uniqueid"`
	Filename string `db:"filename" json:"filename"`
}
