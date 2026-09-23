package pbxv3

import (
	"context"
	"net/http"
)

// The shapes below were taken from the live deployment
// (http://sbc.wevetel.com:8101/wmpbxworker) rather than from the swagger
// document, because swagger describes several of these as a bare object.

// AgentRow is one agent's live state in one queue, from
// GET /secure/callcenter/realtime/agent-status. An agent who is a member of
// three queues is three rows, not one row with three queues.
type AgentRow struct {
	Queue string `json:"queue"`
	// Agent is the bare extension ("5955"), NOT an AMI interface — it is
	// not the value the pause/logoff endpoints want. See AgentPause.
	Agent       string `json:"agent"`
	State       string `json:"state"`
	Duration    int    `json:"duration"`
	CallerID    string `json:"caller_id"`
	Penalty     int    `json:"penalty"`
	CallsTaken  int    `json:"calls_taken"`
	LastCallAgo int    `json:"last_call_ago"`
	// Paused is 0/1 rather than a bool in v3's own JSON.
	Paused int `json:"paused"`
}

type agentStatusResponse struct {
	Agents []AgentRow `json:"agents"`
}

// AgentStatus returns the live per-agent grid.
func (c *Client) AgentStatus(ctx context.Context) ([]AgentRow, error) {
	var out agentStatusResponse
	if err := c.do(ctx, http.MethodGet, "/secure/callcenter/realtime/agent-status", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Agents, nil
}

// Column describes one column of a bucketed report or realtime summary.
// v3's report engine is designer-driven, so the columns are data rather
// than a fixed struct — the caller renders whatever comes back.
type Column struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Type   string `json:"type"`
	Format string `json:"format,omitempty"`
}

// QueueSummary is GET /secure/callcenter/realtime/queue-summary: per-queue
// live stats. Rows are keyed by the Columns' Key values, so they stay
// map[string]any rather than being flattened into a struct that would have
// to be revised every time the designer config changes.
type QueueSummary struct {
	Columns []Column         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
}

func (c *Client) QueueSummary(ctx context.Context) (QueueSummary, error) {
	var out QueueSummary
	err := c.do(ctx, http.MethodGet, "/secure/callcenter/realtime/queue-summary", nil, nil, &out)
	return out, err
}

// WaitingCalls is GET /secure/callcenter/realtime/waiting-calls. Queues is
// null on the wire when nothing is waiting, which unmarshals to a nil map.
type WaitingCalls struct {
	Queues     map[string]any `json:"queues"`
	ServerTime int64          `json:"server_time"`
}

func (c *Client) WaitingCalls(ctx context.Context) (WaitingCalls, error) {
	var out WaitingCalls
	err := c.do(ctx, http.MethodGet, "/secure/callcenter/realtime/waiting-calls", nil, nil, &out)
	return out, err
}

// PauseRequest is the body of POST /realtime/agent/pause.
//
// Interface is the AMI interface as the QUEUE holds it, not the extension
// and not a channel-driver guess — "Local/5955@from-internal" on this PBX,
// "SIP/1001" on a chan_sip one. Sending a string the queue does not hold is
// accepted and silently applies to nothing, so it must be resolved from
// live queue membership.
type PauseRequest struct {
	Interface string `json:"interface"`
	Paused    bool   `json:"paused"`
	// Queue empty means every queue the interface belongs to.
	Queue string `json:"queue,omitempty"`
}

// AgentPause pauses or unpauses an agent (QueuePause).
func (c *Client) AgentPause(ctx context.Context, req PauseRequest) error {
	return c.do(ctx, http.MethodPost, "/secure/callcenter/realtime/agent/pause", nil, req, nil)
}

// LogoffRequest is the body of POST /realtime/agent/logoff. Unlike pause,
// v3 requires the queue here.
type LogoffRequest struct {
	Interface string `json:"interface"`
	Queue     string `json:"queue"`
}

// AgentLogoff removes an agent from a queue (QueueRemove).
func (c *Client) AgentLogoff(ctx context.Context, req LogoffRequest) error {
	return c.do(ctx, http.MethodPost, "/secure/callcenter/realtime/agent/logoff", nil, req, nil)
}

// SpyMode is the fixed allow-list v3 accepts. The names differ from this
// service's own monitor/whisper/barge vocabulary, which is why SpyModeFor
// exists rather than the strings being passed through.
type SpyMode string

const (
	SpyListen SpyMode = "listen" // hear the call only        (monitor)
	SpyCoach  SpyMode = "coach"  // hear it, agent hears you  (whisper)
	SpySteal  SpyMode = "steal"  // take the call over        (barge)
)

// SpyModeFor maps this service's vocabulary onto v3's. The third mode is
// not an exact synonym: "barge" here has meant joining the call, whereas
// v3's "steal" is a Redirect that takes it over, so the agent leg is
// dropped rather than kept.
func SpyModeFor(mode string) (SpyMode, bool) {
	switch mode {
	case "monitor":
		return SpyListen, true
	case "whisper":
		return SpyCoach, true
	case "barge":
		return SpySteal, true
	}
	return "", false
}

// SpyRequest is the body of POST /realtime/spy. The device the spy leg is
// originated FROM comes from v3's own setup table, never from here — a
// caller cannot choose which device to originate from.
type SpyRequest struct {
	Mode SpyMode `json:"mode"`
	// TargetChannel is a live channel name ("SIP/1001-00000001"), not an
	// extension or an interface.
	TargetChannel string `json:"target_channel"`
}

func (c *Client) Spy(ctx context.Context, req SpyRequest) error {
	return c.do(ctx, http.MethodPost, "/secure/callcenter/realtime/spy", nil, req, nil)
}

// Lookups

type lookupQueuesResponse struct {
	Queues []struct {
		Queue string `json:"queue"`
	} `json:"queues"`
}

// LookupQueues returns every queue id the reporting database knows about,
// including the synthetic "ALL"/"NONE" entries v3 includes for filter UIs.
func (c *Client) LookupQueues(ctx context.Context) ([]string, error) {
	var out lookupQueuesResponse
	if err := c.do(ctx, http.MethodGet, "/secure/callcenter/lookups/queues", nil, nil, &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Queues))
	for _, q := range out.Queues {
		names = append(names, q.Queue)
	}
	return names, nil
}

type lookupAgentsResponse struct {
	Agents []struct {
		Agent string `json:"agent"`
	} `json:"agents"`
}

func (c *Client) LookupAgents(ctx context.Context) ([]string, error) {
	var out lookupAgentsResponse
	if err := c.do(ctx, http.MethodGet, "/secure/callcenter/lookups/agents", nil, nil, &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Agents))
	for _, a := range out.Agents {
		names = append(names, a.Agent)
	}
	return names, nil
}
