// Package pbxworker is the HTTP gateway to the pbx-worker service that
// serves historical call-center reports.
//
// Shapes below are CONFIRMED against
// Modules/CallCenter/app/Services/Pbx/PbxReportGateway.php (endpoint,
// params) and its per-method doc comments (row field names). A few row
// fields hinted at elsewhere in the Laravel codebase but not present in
// the gateway's own doc comments (e.g. UnansweredCall's possible
// "last_agent", referenced by UnansweredCallsExport.php) are NOT included
// here — verify against a live response before adding them.
package pbxworker

import "time"

// AnsweredCall is one row from action=answered-calls.
//
// Numeric fields are FlexInt, not int: confirmed live against the real
// worker that e.g. ring_time comes back as a JSON string on some rows and a
// JSON number on others (see flexint.go) — a plain int field decodes some
// rows and hard-fails json.Unmarshal for the WHOLE response on others.
type AnsweredCall struct {
	Datetime      string  `json:"datetime"`
	QueueName     string  `json:"queue_name"`
	AgentName     string  `json:"agent_name"`
	Event         string  `json:"event"`
	UniqueID      string  `json:"uniqueid"`
	CallerID      string  `json:"caller_id"`
	URL           string  `json:"url"`
	DID           string  `json:"did"`
	RingTime      FlexInt `json:"ring_time"`
	RecordingFile string  `json:"recording_file"`
	HoldTime      FlexInt `json:"hold_time"`
	Duration      FlexInt `json:"duration"`
	Position      FlexInt `json:"position"`
	TransferExten string  `json:"transfer_exten"`
	YearMonth     string  `json:"year_month"`
	YearWeek      string  `json:"year_week"`
	Date          string  `json:"date"`
	Hour          FlexInt `json:"hour"`
	DayOfWeek     FlexInt `json:"day_of_week"`
	SecondsOfDay  FlexInt `json:"seconds_of_day"`
}

// UnansweredCall is one row from action=unanswered-calls. Event is one of
// the four real CallEvent enum cases for unanswered calls: ABANDON,
// EXITWITHTIMEOUT, EXITWITHKEY, EXITEMPTY. See AnsweredCall's own doc
// comment for why the numeric fields are FlexInt.
type UnansweredCall struct {
	Datetime  string  `json:"datetime"`
	QueueName string  `json:"queue_name"`
	AgentName string  `json:"agent_name"`
	Event     string  `json:"event"`
	UniqueID  string  `json:"uniqueid"`
	CallerID  string  `json:"caller_id"`
	URL       string  `json:"url"`
	DID       string  `json:"did"`
	RingTime  FlexInt `json:"ring_time"`
	HoldTime  FlexInt `json:"hold_time"`
	YearMonth string  `json:"year_month"`
	YearWeek  string  `json:"year_week"`
	Date      string  `json:"date"`
	Hour      FlexInt `json:"hour"`
	DayOfWeek FlexInt `json:"day_of_week"`
}

// AgentEvent is one row from action=agent-events.
type AgentEvent struct {
	Datetime  string `json:"datetime"`
	QueueName string `json:"queue_name"`
	AgentName string `json:"agent_name"`
	Event     string `json:"event"`
	Info1     string `json:"info1"`
	Info2     string `json:"info2"`
	Info3     string `json:"info3"`
	Timestamp string `json:"timestamp"`
	UniqueID  string `json:"uniqueid"`
}

// CallSearchResult is one row from action=call-search. See AnsweredCall's
// own doc comment for why the numeric fields are FlexInt.
type CallSearchResult struct {
	UniqueID      string  `json:"uniqueid"`
	CallerID      string  `json:"caller_id"`
	DateStart     string  `json:"date_start"`
	DateEnd       string  `json:"date_end"`
	Event         string  `json:"event"`
	AgentName     string  `json:"agent_name"`
	QueueName     string  `json:"queue_name"`
	TalkTime      FlexInt `json:"talk_time"`
	TotalDuration FlexInt `json:"total_duration"`
	WaitTime      FlexInt `json:"wait_time"`
	QueueHops     FlexInt `json:"queue_hops"`
	RecordingFile string  `json:"recording_file"`
}

// CallDetailRow is one timeline event for a single call. action=call-detail
// returns an ARRAY of these (one call can have many rows across its
// lifecycle) — not a single summary object.
type CallDetailRow struct {
	Datetime      string `json:"datetime"`
	QueueName     string `json:"queue_name"`
	AgentName     string `json:"agent_name"`
	Event         string `json:"event"`
	Info1         string `json:"info1"`
	Info2         string `json:"info2"`
	Info3         string `json:"info3"`
	UniqueID      string `json:"uniqueid"`
	RecordingFile string `json:"recording_file"`
}

// ListParams covers the filters PbxReportGateway::buildParams() always
// sends for answered-calls/unanswered-calls/agent-events: a date range, a
// seconds-of-day sub-range within each day, and optional queue/agent name
// filters. See hmac_client.go for exact wire format (naive local
// "Y-m-d H:i:s" strings, NOT RFC3339).
type ListParams struct {
	DateFrom     time.Time
	DateTo       time.Time
	SecondsStart int // default 0
	SecondsEnd   int // default 86399
	Queues       []string
	Agents       []string
}

// CallSearchParams covers call-search's extra filters on top of ListParams.
// DurationOperator/DurationSeconds are only ever sent together —
// PbxReportGateway.php only includes them as a pair.
type CallSearchParams struct {
	ListParams
	CallerID         string
	UniqueID         string
	DurationOperator string
	DurationSeconds  *int
}
