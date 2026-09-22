// Package calllog is the browser softphone's own record of a call —
// created when a call starts, updated when it is answered, finalized when
// it ends. It is the direct port of
// Modules/SoftPhone/app/Livewire/Softphone.php's createCallLog /
// markCallAnswered / finalizeCallLog / detectQueue methods: same
// reconciliation rules against whatever the PBX worker's own poller
// already wrote for the same call, same status-downgrade guard, same
// queue-detection fallback chain.
//
// # Why this needs reconciliation at all
//
// Two independent things can create a row for the same call: this
// service (the moment the browser sees a call start) and the PBX worker's
// own poller (on its own schedule, from the PBX's call state, not the
// browser's). Whichever gets there first stands; the other MERGES into
// it rather than creating a duplicate row for the same call, using
// unique_id / channel / caller_id+queue-within-a-window as the same-call
// signal the PHP original uses. There is no PBX worker poller writing into
// this Go service's own database (that poller, if it exists at all,
// belongs to whoever owns qstats), so in this product the row this service
// creates IS the record — but the matching logic stays in exactly because
// a client-side retry (the browser calling createCallLog twice for one
// ring) must not create two rows for one call.
package calllog

import (
	"database/sql"
	"time"
)

// Direction values, matching CallLog::$fillable's direction column.
// Softphone.php's createCallLog() normalises 'incoming'/'queue' -> inbound
// and 'outgoing' -> outbound before writing; this package expects the
// caller to have already normalised (see handlers/calllog.go).
const (
	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"
)

// Status values. "ongoing" is the row's state between create and
// answer/end; the rest are terminal.
const (
	StatusOngoing   = "ongoing"
	StatusAnswered  = "answered"
	StatusCompleted = "completed"
	StatusMissed    = "missed"
	StatusAbandoned = "abandoned"
)

// CallLog mirrors Modules/SoftPhone/app/Models/CallLog.php's $fillable
// columns exactly — table call_logs, see migrations/callcenter/001_init.sql.
type CallLog struct {
	ID                 string         `db:"id"`
	UserID             sql.NullString `db:"user_id"`
	Extension          sql.NullString `db:"extension"`
	CallID             sql.NullString `db:"call_id"`
	UniqueID           sql.NullString `db:"unique_id"`
	Channel            sql.NullString `db:"channel"`
	Direction          string         `db:"direction"`
	CallerID           sql.NullString `db:"caller_id"`
	CallerName         sql.NullString `db:"caller_name"`
	Destination        sql.NullString `db:"destination"`
	Queue              sql.NullString `db:"queue"`
	Status             string         `db:"status"`
	WaitSeconds        int            `db:"wait_seconds"`
	TalkSeconds        int            `db:"talk_seconds"`
	StartedAt          sql.NullTime   `db:"started_at"`
	AnsweredAt         sql.NullTime   `db:"answered_at"`
	EndedAt            sql.NullTime   `db:"ended_at"`
	Metadata           sql.NullString `db:"metadata"` // raw JSON text
	RecordingFile      sql.NullString `db:"recording_file"`
	RecordingFormat    sql.NullString `db:"recording_format"`
	RecordingFetchedAt sql.NullTime   `db:"recording_fetched_at"`
	CreatedAt          time.Time      `db:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at"`
}

// HasRecording mirrors CallLog::hasRecording().
func (c *CallLog) HasRecording() bool {
	return c.RecordingFile.Valid && c.RecordingFile.String != ""
}
