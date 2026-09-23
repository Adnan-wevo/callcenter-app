package pbxv3

import (
	"context"
	"net/http"
)

// v3's AMI control surface (/secure/ami/...).
//
// Preferred over /callcenter/realtime/spy for anything supervisory: that one
// takes a `target_channel` — a live channel name like "SIP/601-0000001" —
// and nothing in v3 resolves an extension to its current channel, so a
// caller holding only an extension cannot use it. These endpoints take
// `target_ext` and do that resolution themselves.
//
// `agent_ext` is sent explicitly on every call rather than left to default.
// v3 falls back to the extension of whoever the token belongs to, and this
// service authenticates as a single service account that owns no extension
// — so omitting it would act on nobody, or on the wrong person.

// ConferenceStart pulls target_ext into a conference with agent_ext's
// current call.
func (c *Client) ConferenceStart(ctx context.Context, agentExt, targetExt string) error {
	body := map[string]string{"agent_ext": agentExt, "target_ext": targetExt}
	return c.do(ctx, http.MethodPost, "/secure/ami/conference/start", nil, body, nil)
}

// ConferenceEnd tears the whole conference down for everyone.
func (c *Client) ConferenceEnd(ctx context.Context, agentExt string) error {
	body := map[string]string{"agent_ext": agentExt}
	return c.do(ctx, http.MethodPost, "/secure/ami/conference/end", nil, body, nil)
}

// ConferenceLeave removes just this agent, leaving the others talking.
func (c *Client) ConferenceLeave(ctx context.Context, agentExt string) error {
	body := map[string]string{"agent_ext": agentExt}
	return c.do(ctx, http.MethodPost, "/secure/ami/conference/leave", nil, body, nil)
}

// ConferenceMute mutes or unmutes this agent within the conference. state is
// "on" or "off".
func (c *Client) ConferenceMute(ctx context.Context, agentExt string, muted bool) error {
	state := "off"
	if muted {
		state = "on"
	}
	body := map[string]string{"agent_ext": agentExt, "state": state}
	return c.do(ctx, http.MethodPost, "/secure/ami/conference/mute", nil, body, nil)
}

// SupervisionMode is what /secure/ami/supervision/start accepts.
type SupervisionMode string

const (
	// SuperviseMonitor listens only; the agent cannot hear the supervisor.
	SuperviseMonitor SupervisionMode = "monitor"
	// SuperviseBarge joins the call so everyone hears the supervisor.
	SuperviseBarge SupervisionMode = "barge"
)

// SupervisionStart begins monitoring or barging targetExt's live call.
// Whisper is NOT a mode here — it has its own endpoint (see WhisperStart).
func (c *Client) SupervisionStart(ctx context.Context, mode SupervisionMode, targetExt string) error {
	body := map[string]string{"mode": string(mode), "target_ext": targetExt}
	return c.do(ctx, http.MethodPost, "/secure/ami/supervision/start", nil, body, nil)
}

// WhisperStart coaches targetExt: the supervisor is heard by the agent and
// not by the caller.
func (c *Client) WhisperStart(ctx context.Context, targetExt string) error {
	body := map[string]string{"target_ext": targetExt}
	return c.do(ctx, http.MethodPost, "/secure/ami/whisper/start", nil, body, nil)
}

// TransferBlind hands agentExt's current call to extension and drops the
// agent out of it.
func (c *Client) TransferBlind(ctx context.Context, agentExt, extension string) error {
	body := map[string]string{"agent_ext": agentExt, "extension": extension}
	return c.do(ctx, http.MethodPost, "/secure/ami/transfer/blind", nil, body, nil)
}

// TransferAttended dials extension so the agent can speak to them first,
// with the caller on hold. Completing or cancelling it is a further step
// (/secure/ami/transfer/attended/cancel).
func (c *Client) TransferAttended(ctx context.Context, agentExt, extension string) error {
	body := map[string]string{"agent_ext": agentExt, "extension": extension}
	return c.do(ctx, http.MethodPost, "/secure/ami/transfer/attended", nil, body, nil)
}

func (c *Client) TransferAttendedCancel(ctx context.Context, agentExt string) error {
	body := map[string]string{"agent_ext": agentExt}
	return c.do(ctx, http.MethodPost, "/secure/ami/transfer/attended/cancel", nil, body, nil)
}
