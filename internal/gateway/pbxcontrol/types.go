// Package pbxcontrol is the HTTP client for pbx-worker's LIVE control
// surface — queue/agent/call snapshots and AMI command dispatch — as
// distinct from internal/gateway/pbxworker, which talks to the same host's
// HISTORICAL reporting endpoint (/api/reports.php).
//
// Both packages target the same pbx-worker deployment with the SAME
// credentials: Modules/SoftPhone/config/softphone.php and
// Modules/CallCenter/app/Services/Pbx/PbxReportGateway.php both resolve
// PBX_WORKER_URL/PBX_WORKER_API_KEY/PBX_WORKER_SECRET from the same env
// vars in heal-crm. This package is confirmed live against the real
// staging worker (sbc.wevetel.com) — every shape below was read from an
// actual response, not inferred from PHP source alone.
package pbxcontrol

import (
	"bytes"
	"encoding/json"
)

// FlexString unmarshals a JSON string, number, boolean or null into a Go
// string. pbx-worker's PHP source is loose about this: a queue/agent whose
// name was never explicitly set is echoed back as its array key, which PHP
// happily lets be an int — the SAME field (e.g. "extension", queue "name")
// comes back as a JSON string on one row and a JSON number on another
// within a single response. A plain `string` field decodes correctly on
// some rows and returns a hard json.Unmarshal error on others — this type
// exists so a rename or reshuffle upstream produces a wrong-looking value
// here, not a decode failure that takes the whole response down with it.
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		*f = ""
		return nil
	}
	// A quoted string decodes straight through.
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = FlexString(s)
		return nil
	}
	// Fall back to the raw token (number, true/false) as its literal text —
	// "40001", "true" — rather than attempting to be clever about what it
	// "means". This is a display/passthrough value, never parsed back into
	// a number by this client.
	*f = FlexString(bytes.Trim(b, `"`))
	return nil
}

func (f FlexString) String() string { return string(f) }

// QueueMember is one agent's membership row within a queue snapshot.
type QueueMember struct {
	Interface       string     `json:"interface"`
	Name            string     `json:"name"`
	StatusCode      int        `json:"status_code"`
	StatusText      string     `json:"status_text"`
	Paused          bool       `json:"paused"`
	PauseReason     FlexString `json:"pause_reason"`
	CallsTakenToday int        `json:"calls_taken_today"`
	LastCallEpoch   int64      `json:"last_call_epoch"`
}

// Queue is one queue's row from action=queues (file=queues).
type Queue struct {
	Name           FlexString    `json:"name"`
	Strategy       string        `json:"strategy"`
	CallsWaiting   int           `json:"calls_waiting"`
	CompletedToday int           `json:"completed_today"`
	AbandonedToday int           `json:"abandoned_today"`
	SLAPercent     *float64      `json:"sla_percent"`
	AvgHoldSeconds *float64      `json:"avg_hold_seconds"`
	MaxHoldSeconds *float64      `json:"max_hold_seconds"`
	AvgTalkSeconds float64       `json:"avg_talk_seconds"`
	Members        []QueueMember `json:"members"`
	Callers        []QueueCaller `json:"callers"`
}

// QueueCaller is one caller currently waiting in a queue.
type QueueCaller struct {
	CallerID    FlexString `json:"caller_id"`
	Position    int        `json:"position"`
	WaitSeconds int        `json:"wait_seconds"`
	UniqueID    string     `json:"unique_id"`
}

// QueueSnapshot is the data payload of file=queues.
type QueueSnapshot struct {
	Queues map[string]Queue `json:"queues"`
}

// AgentQueueDetail is one queue's pause state for one agent, keyed by
// queue id in Agent.QueueDetails.
type AgentQueueDetail struct {
	Paused      bool       `json:"paused"`
	PauseReason FlexString `json:"pause_reason"`
}

// Agent is one row from file=agents.
type Agent struct {
	Extension       FlexString                  `json:"extension"`
	Status          string                      `json:"status"`
	Paused          bool                        `json:"paused"`
	PauseReason     FlexString                  `json:"pause_reason"`
	Queues          []int                       `json:"queues"`
	QueueDetails    map[string]AgentQueueDetail `json:"queue_details"`
	CallsTakenToday int                         `json:"calls_taken_today"`
	ActiveCall      *AgentActiveCall            `json:"active_call"`
	LastEventEpoch  int64                       `json:"last_event_epoch"`
}

type AgentActiveCall struct {
	UniqueID string `json:"unique_id"`
	CallerID string `json:"caller_id"`
	Queue    string `json:"queue"`
}

// AgentSnapshot is the data payload of file=agents.
type AgentSnapshot struct {
	Agents map[string]Agent `json:"agents"`
}

// LiveCall is one row from file=calls — a channel currently in progress,
// not yet a completed CDR.
type LiveCall struct {
	UniqueID        string     `json:"unique_id"`
	CallerID        FlexString `json:"caller_id"`
	Queue           *string    `json:"queue"`
	State           string     `json:"state"`
	DurationSeconds int        `json:"duration_seconds"`
	AgentExt        *string    `json:"agent_ext"`
	AnsweredEpoch   *int64     `json:"answered_epoch"`
	Channel         string     `json:"channel"`
}

// CallsSnapshot is the data payload of file=calls.
type CallsSnapshot struct {
	Calls []LiveCall `json:"calls"`
}

// HealthSnapshot is the data payload of file=health — worker process
// health, the command queue depth, and qstats poller staleness.
type HealthSnapshot struct {
	Worker struct {
		Status                 string `json:"status"`
		PID                    int    `json:"pid"`
		StartedAt              int64  `json:"started_at"`
		UptimeSeconds          int64  `json:"uptime_seconds"`
		AMIConnected           bool   `json:"ami_connected"`
		LastAMIEventEpoch      int64  `json:"last_ami_event_epoch"`
		LastSnapshotWriteEpoch int64  `json:"last_snapshot_write_epoch"`
		SnapshotWriteCount     int64  `json:"snapshot_write_count"`
		ReconnectCount         int    `json:"reconnect_count"`
		Version                string `json:"version"`
	} `json:"worker"`
	CommandQueue struct {
		Pending        int `json:"pending"`
		Processing     int `json:"processing"`
		FailedLastHour int `json:"failed_last_hour"`
	} `json:"command_queue"`
	Qstats struct {
		Enabled            bool   `json:"enabled"`
		SnapshotAgeSeconds *int   `json:"snapshot_age_seconds"`
		LastUpdatedEpoch   int64  `json:"last_updated_epoch"`
		Status             string `json:"status"`
	} `json:"qstats"`
}

// --- Commands ------------------------------------------------------------
//
// Payload field names and validation rules below are taken verbatim from
// wevetel-pbx-worker/src/CommandProcessor.php's validate*() methods — that
// file is the actual server-side contract; the Go structs mirror its field
// names exactly so a request this client builds either passes or fails the
// SAME checks the PHP worker enforces, with the same field name in the
// error either way.

// QueueLoginPayload logs an agent into one or more queues.
// CommandProcessor::validateQueueMembership.
type QueueLoginPayload struct {
	// Interface must match ^(SIP|PJSIP|Local)/[a-zA-Z0-9_-]+(@[a-zA-Z0-9_-]+)?$
	Interface string `json:"interface"`
	// Queues: use ["all"] to mean "every queue this agent is currently a
	// member of" — the worker resolves that from its own state, it does
	// NOT mean "every queue in the system" (CommandProcessor.execQueueLogin's
	// own comment is explicit about this — an earlier version of the worker
	// got this wrong and corrupted queue_log reports).
	Queues []string `json:"queues"`
	// StateInterface lets a Local/ member report its underlying device's
	// real state to Asterisk. Optional; defaults to Interface on the worker
	// if omitted.
	StateInterface string `json:"state_interface,omitempty"`
}

// QueueLogoutPayload logs an agent out of one or more queues. Same shape
// and same "all" semantics as QueueLoginPayload.
type QueueLogoutPayload struct {
	Interface string   `json:"interface"`
	Queues    []string `json:"queues"`
}

// PausePayload pauses or unpauses an agent (action name carries which).
// CommandProcessor::validateInterface.
type PausePayload struct {
	Interface string `json:"interface"`
	Queue     string `json:"queue,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// RedirectPayload blind-transfers a queued (not yet answered) caller to an
// extension. CommandProcessor::validateRedirect — exactly one of Channel /
// AgentExt is required; Extension is always required.
type RedirectPayload struct {
	Channel   string `json:"channel,omitempty"`
	AgentExt  string `json:"agent_ext,omitempty"`
	Extension string `json:"extension"`
	Context   string `json:"context,omitempty"`
}

// PickupPayload rings a caller's channel directly at an agent's extension.
// CommandProcessor::validatePickup — both fields required.
type PickupPayload struct {
	Channel  string `json:"channel"`
	AgentExt string `json:"agent_ext"`
	Context  string `json:"context,omitempty"`
}

// SpyMode is validated server-side against exactly these three values
// (CommandProcessor::validateSpy).
type SpyMode string

const (
	SpyModeMonitor SpyMode = "monitor" // listen only
	SpyModeWhisper SpyMode = "whisper" // supervisor heard by agent only
	SpyModeBarge   SpyMode = "barge"   // supervisor heard by both parties
)

// SpyPayload originates a supervisor channel into ChanSpy/ExtenSpy on a
// target agent's live channel. CommandProcessor::validateSpy.
type SpyPayload struct {
	SupervisorChannel string  `json:"supervisor_channel"`
	TargetExt         string  `json:"target_ext"`
	Mode              SpyMode `json:"mode,omitempty"`
}

// HangupPayload ends a channel. CommandProcessor::validateHangup.
type HangupPayload struct {
	Channel string `json:"channel"`
	// Cause is a numeric Q.850 cause code (e.g. 16 = normal clearing),
	// sent as a string here because the worker's own validation regex
	// (\d{1,3}) is applied to whatever this serialises to.
	Cause string `json:"cause,omitempty"`
}

// AttendedTransferPayload starts an attended transfer.
// CommandProcessor::validateAtxfer — exactly one of Channel / AgentExt is
// required; Extension is always required.
type AttendedTransferPayload struct {
	Channel   string `json:"channel,omitempty"`
	AgentExt  string `json:"agent_ext,omitempty"`
	Extension string `json:"extension"`
	Context   string `json:"context,omitempty"`
}

// CommandResult is the terminal shape of a polled command — both the
// "done" and "failed" envelopes, normalised into one Go type. Status is
// "success" or "failed"; Message/Error/Detail are populated depending on
// which and on the action (see CommandProcessor's individual exec*
// methods — e.g. only queue-login/logout populate Detail).
type CommandResult struct {
	CommandID string            `json:"command_id"`
	Status    string            `json:"status"`
	Action    string            `json:"action,omitempty"`
	Message   string            `json:"message,omitempty"`
	Error     string            `json:"error,omitempty"`
	Detail    map[string]string `json:"detail,omitempty"`
	// CompletedAt is a Unix epoch, per moveDone/moveFailed in
	// CommandProcessor.php.
	CompletedAt int64 `json:"completed_at,omitempty"`
}

// IsTerminal reports whether Status is a final state (success/failed),
// as opposed to queued/processing (still in flight) or timeout (this
// client gave up polling — the worker may still complete it later).
func (r CommandResult) IsTerminal() bool {
	return r.Status == "success" || r.Status == "failed"
}
