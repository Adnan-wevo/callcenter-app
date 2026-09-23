package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/gateway/pbxcontrol"
	"callcenter-service/internal/middleware"
)

// dispatchAndRespond runs a command through pbxcontrol's submit+poll
// orchestration and writes the HTTP response. This is the one place that
// turns a CommandResult into an API response, so every command endpoint
// answers the same shape regardless of action.
func (h *Handlers) dispatchAndRespond(c *gin.Context, action string, payload any) {
	if h.pbxControl == nil {
		apires.Error(c, http.StatusServiceUnavailable, "PBX control is not configured", nil)
		return
	}

	result, err := h.pbxControl.Dispatch(c.Request.Context(), action, payload)
	if err != nil {
		// Submit itself failed (couldn't even queue the command) — the
		// worker is unreachable or rejected the request outright.
		apires.Error(c, http.StatusBadGateway, "could not reach the PBX worker", nil)
		return
	}

	switch result.Status {
	case "success":
		apires.Item(c, http.StatusOK, gin.H{
			"command_id": result.CommandID,
			"message":    result.Message,
			"detail":     result.Detail,
		})
	case "timeout":
		// Matches PbxCommandService::poll()'s own semantics: the command may
		// still complete worker-side after this response — this is not a
		// failure, it is "we stopped waiting".
		apires.Item(c, http.StatusAccepted, gin.H{
			"command_id": result.CommandID,
			"status":     "timeout",
			"message":    result.Message,
		})
	default: // "failed", "not_found"
		apires.Error(c, http.StatusUnprocessableEntity, result.Error, gin.H{
			"command_id": result.CommandID,
			"status":     result.Status,
		})
	}
}

// interfaceFor builds the AMI Interface string (e.g. "PJSIP/1001") for the
// calling agent's own extension — queue login/logout/pause/unpause always
// act on the CALLER's own membership, never an interface named in the
// request body, for the same reason the callback write-path takes the
// agent id from the token (see handlers/callback.go): an endpoint that took
// an arbitrary interface would let any agent pause someone else's queue
// membership.
func (h *Handlers) interfaceFor(c *gin.Context) (string, bool) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return "", false
	}
	ext, ok := h.softphone.ExtensionFor(c.Request.Context(), user.ID)
	if !ok || ext == "" {
		apires.Error(c, http.StatusNotFound, "no SIP extension is assigned to your account", nil)
		return "", false
	}
	return h.queueInterfaceFor(c.Request.Context(), ext), true
}

// queueInterfaceFor resolves the AMI Interface string for ext by looking it
// up in the live queue snapshot, rather than assembling one from a channel
// driver prefix.
//
// This used to return "PJSIP/" + ext on the reasoning that PJSIP is the
// modern driver. That was wrong for this deployment and silently broke
// every queue pause/unpause/logout: AMI matches the Interface against the
// member string the queue actually holds, and on this PBX that is
//
//	name:      "Local/5955@from-internal"
//	interface: "5955@from-internal"
//
// (verified against the live queue 50000 snapshot). "PJSIP/5955" matches no
// member, so Asterisk accepted the command and applied it to nothing — the
// agent saw a button that appeared to work and a pause that never happened.
// The queue's own membership is the only reliable source here, because the
// format is a property of how the member was ADDED, not of the extension:
// heal-crm's own provisioning writes chan_sip peers ("SIP/<ext>", see
// OwnDeviceRepository.go in the reference source) while this PBX registers
// queue members as Local channels.
//
// The fallback matters only for the one case with nothing to look up —
// joining a queue this agent is not yet a member of anywhere — and matches
// what this PBX uses.
func (h *Handlers) queueInterfaceFor(ctx context.Context, ext string) string {
	fallback := "Local/" + ext + "@from-internal"
	if h.pbxControl == nil {
		return fallback
	}
	snapshot, _, err := h.pbxControl.Queues(ctx)
	if err != nil {
		return fallback
	}
	for _, queue := range snapshot.Queues {
		for _, member := range queue.Members {
			if !interfaceIsExtension(member, ext) {
				continue
			}
			// Name is the full channel ("Local/5955@from-internal");
			// Interface is the same thing with the driver prefix already
			// stripped, so it is not what AMI wants on its own.
			if member.Name != "" {
				return member.Name
			}
			return member.Interface
		}
	}
	return fallback
}

// interfaceIsExtension reports whether a queue member row belongs to ext,
// matching on the leading number so "5955@from-internal",
// "Local/5955@from-internal" and "SIP/5955" all resolve for extension 5955
// without "5955" also matching "59550".
func interfaceIsExtension(member pbxcontrol.QueueMember, ext string) bool {
	for _, candidate := range []string{member.Interface, member.Name} {
		if candidate == "" {
			continue
		}
		if s := strings.TrimPrefix(candidate, "Local/"); s != candidate {
			candidate = s
		}
		if i := strings.IndexAny(candidate, "/"); i >= 0 {
			candidate = candidate[i+1:]
		}
		if i := strings.IndexAny(candidate, "@"); i >= 0 {
			candidate = candidate[:i]
		}
		if candidate == ext {
			return true
		}
	}
	return false
}

type queueLoginBody struct {
	// Queues: omit or send ["all"] for "every queue I'm currently a member
	// of" — see pbxcontrol.QueueLoginPayload's own doc comment for why that
	// is NOT "every queue in the system".
	Queues []string `json:"queues"`
}

// POST /api/v1/secure/softphone/queue/login
func (h *Handlers) QueueLogin(c *gin.Context) {
	iface, ok := h.interfaceFor(c)
	if !ok {
		return
	}
	var body queueLoginBody
	_ = c.ShouldBindJSON(&body)
	if len(body.Queues) == 0 {
		body.Queues = []string{"all"}
	}
	h.dispatchAndRespond(c, "queue-login", pbxcontrol.QueueLoginPayload{
		Interface: iface,
		Queues:    body.Queues,
	})
}

// POST /api/v1/secure/softphone/queue/logout
func (h *Handlers) QueueLogout(c *gin.Context) {
	iface, ok := h.interfaceFor(c)
	if !ok {
		return
	}
	var body queueLoginBody
	_ = c.ShouldBindJSON(&body)
	if len(body.Queues) == 0 {
		body.Queues = []string{"all"}
	}
	h.dispatchAndRespond(c, "queue-logout", pbxcontrol.QueueLogoutPayload{
		Interface: iface,
		Queues:    body.Queues,
	})
}

type pauseBody struct {
	Queue  string `json:"queue"`
	Reason string `json:"reason"`
}

// POST /api/v1/secure/softphone/queue/pause
func (h *Handlers) QueuePause(c *gin.Context) {
	iface, ok := h.interfaceFor(c)
	if !ok {
		return
	}
	var body pauseBody
	_ = c.ShouldBindJSON(&body)
	h.dispatchAndRespond(c, "pause", pbxcontrol.PausePayload{
		Interface: iface,
		Queue:     body.Queue,
		Reason:    body.Reason,
	})
}

// POST /api/v1/secure/softphone/queue/unpause
func (h *Handlers) QueueUnpause(c *gin.Context) {
	iface, ok := h.interfaceFor(c)
	if !ok {
		return
	}
	var body pauseBody
	_ = c.ShouldBindJSON(&body)
	h.dispatchAndRespond(c, "unpause", pbxcontrol.PausePayload{
		Interface: iface,
		Queue:     body.Queue,
	})
}

type agentPauseBody struct {
	Interface string `json:"interface" binding:"required"`
	Queue     string `json:"queue"`
	Reason    string `json:"reason"`
}

// POST /api/v1/secure/softphone/queue/agent-pause — supervisor only.
//
// The self-service /queue/pause above is deliberately locked to the
// CALLER's own interface (see interfaceFor's doc comment) so an ordinary
// agent endpoint can never pause someone else. This is the Realtime
// Monitor's own action — a supervisor acting on an arbitrary agent — so it
// is a separate, separately-gated endpoint rather than a body field bolted
// onto the self-service one, the same split redirect/pickup/spy already
// use for the same reason.
func (h *Handlers) QueueAgentPause(c *gin.Context) {
	var body agentPauseBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "interface is required", nil)
		return
	}
	h.dispatchAndRespond(c, "pause", pbxcontrol.PausePayload{
		Interface: body.Interface,
		Queue:     body.Queue,
		Reason:    body.Reason,
	})
}

// POST /api/v1/secure/softphone/queue/agent-unpause — supervisor only.
func (h *Handlers) QueueAgentUnpause(c *gin.Context) {
	var body agentPauseBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "interface is required", nil)
		return
	}
	h.dispatchAndRespond(c, "unpause", pbxcontrol.PausePayload{
		Interface: body.Interface,
		Queue:     body.Queue,
	})
}

type agentLogoutBody struct {
	Interface string   `json:"interface" binding:"required"`
	Queues    []string `json:"queues"`
}

// POST /api/v1/secure/softphone/queue/agent-logout — supervisor only.
func (h *Handlers) QueueAgentLogout(c *gin.Context) {
	var body agentLogoutBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "interface is required", nil)
		return
	}
	if len(body.Queues) == 0 {
		body.Queues = []string{"all"}
	}
	h.dispatchAndRespond(c, "queue-logout", pbxcontrol.QueueLogoutPayload{
		Interface: body.Interface,
		Queues:    body.Queues,
	})
}

type redirectBody struct {
	Channel   string `json:"channel"`
	AgentExt  string `json:"agent_ext"`
	Extension string `json:"extension" binding:"required"`
}

// POST /api/v1/secure/softphone/queue/redirect — supervisor only.
func (h *Handlers) QueueRedirect(c *gin.Context) {
	var body redirectBody
	if err := c.ShouldBindJSON(&body); err != nil || (body.Channel == "" && body.AgentExt == "") {
		apires.Error(c, http.StatusUnprocessableEntity, "channel or agent_ext, and extension, are required", nil)
		return
	}
	h.dispatchAndRespond(c, "redirect", pbxcontrol.RedirectPayload{
		Channel:   body.Channel,
		AgentExt:  body.AgentExt,
		Extension: body.Extension,
	})
}

type pickupBody struct {
	Channel  string `json:"channel" binding:"required"`
	AgentExt string `json:"agent_ext" binding:"required"`
}

// POST /api/v1/secure/softphone/queue/pickup — supervisor only.
func (h *Handlers) QueuePickup(c *gin.Context) {
	var body pickupBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "channel and agent_ext are required", nil)
		return
	}
	h.dispatchAndRespond(c, "pickup", pbxcontrol.PickupPayload{
		Channel:  body.Channel,
		AgentExt: body.AgentExt,
	})
}

type spyBody struct {
	TargetExt string             `json:"target_ext" binding:"required"`
	Mode      pbxcontrol.SpyMode `json:"mode"`
}

// POST /api/v1/secure/softphone/queue/spy — supervisor only.
//
// The supervisor's OWN channel is used, never one named in the request —
// same reasoning as interfaceFor: a spy endpoint that let the caller name
// an arbitrary supervisor_channel would let any supervisor-permissioned
// agent originate a spy session as if FROM someone else's extension.
func (h *Handlers) QueueSpy(c *gin.Context) {
	iface, ok := h.interfaceFor(c)
	if !ok {
		return
	}
	var body spyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "target_ext is required", nil)
		return
	}
	if body.Mode == "" {
		body.Mode = pbxcontrol.SpyModeMonitor
	}
	h.dispatchAndRespond(c, "spy", pbxcontrol.SpyPayload{
		SupervisorChannel: iface,
		TargetExt:         body.TargetExt,
		Mode:              body.Mode,
	})
}

// GET /api/v1/secure/softphone/queues — live snapshot, not historical.
func (h *Handlers) LiveQueues(c *gin.Context) {
	h.liveSnapshot(c, func(ctx *gin.Context) (any, pbxcontrol.Snapshot, error) {
		return h.pbxControl.Queues(ctx.Request.Context())
	})
}

// GET /api/v1/secure/softphone/agents
func (h *Handlers) LiveAgents(c *gin.Context) {
	h.liveSnapshot(c, func(ctx *gin.Context) (any, pbxcontrol.Snapshot, error) {
		return h.pbxControl.Agents(ctx.Request.Context())
	})
}

// GET /api/v1/secure/softphone/live-calls
func (h *Handlers) LiveCalls(c *gin.Context) {
	h.liveSnapshot(c, func(ctx *gin.Context) (any, pbxcontrol.Snapshot, error) {
		return h.pbxControl.Calls(ctx.Request.Context())
	})
}

func (h *Handlers) liveSnapshot(c *gin.Context, fetch func(*gin.Context) (any, pbxcontrol.Snapshot, error)) {
	if h.pbxControl == nil {
		apires.Error(c, http.StatusServiceUnavailable, "PBX control is not configured", nil)
		return
	}
	data, snap, err := fetch(c)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "could not reach the PBX worker", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{
		"data": data,
		"meta": gin.H{
			"stale":        snap.Stale,
			"age_seconds":  snap.AgeSeconds,
			"generated_at": snap.GeneratedAt,
		},
	})
}
