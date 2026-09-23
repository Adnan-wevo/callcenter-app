package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/gateway/pbxv3"
	"callcenter-service/internal/middleware"
)

// Conference, whisper and attended transfer, over v3's AMI surface.
//
// These have no v2 counterpart to fall back to: v2's command queue offers
// spy/redirect/pickup and nothing else, so conference and attended transfer
// simply did not exist before. Unavailable rather than degraded when v3 is
// not configured.

// callerExtension resolves the calling agent's own extension, which every
// endpoint here acts as. v3 would otherwise fall back to the extension of
// whoever the token belongs to — this service's single service account,
// which owns none.
func (h *Handlers) callerExtension(c *gin.Context) (string, bool) {
	if h.pbxV3 == nil {
		apires.Error(c, http.StatusServiceUnavailable,
			"this needs pbx-worker v3 to be configured", nil)
		return "", false
	}
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
	return ext, true
}

// respondAMI turns an AMI call's outcome into a response, so every endpoint
// here answers the same shape.
func respondAMI(c *gin.Context, err error, status string) {
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "the PBX refused that request", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": status})
}

type targetExtBody struct {
	TargetExt string `json:"target_ext" binding:"required"`
}

// POST /api/v1/secure/softphone/conference/start
//
// Adds target_ext to the call this agent is already on. Not gated by
// softphone.supervise: it acts on the CALLER's own call, the same boundary
// queue pause and self-service transfer already sit behind.
func (h *Handlers) ConferenceStart(c *gin.Context) {
	ext, ok := h.callerExtension(c)
	if !ok {
		return
	}
	var body targetExtBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "target_ext is required", nil)
		return
	}
	respondAMI(c, h.pbxV3.ConferenceStart(c.Request.Context(), ext, body.TargetExt), "conference started")
}

// POST /api/v1/secure/softphone/conference/end — ends it for everyone.
func (h *Handlers) ConferenceEnd(c *gin.Context) {
	ext, ok := h.callerExtension(c)
	if !ok {
		return
	}
	respondAMI(c, h.pbxV3.ConferenceEnd(c.Request.Context(), ext), "conference ended")
}

// POST /api/v1/secure/softphone/conference/leave — leaves, others continue.
func (h *Handlers) ConferenceLeave(c *gin.Context) {
	ext, ok := h.callerExtension(c)
	if !ok {
		return
	}
	respondAMI(c, h.pbxV3.ConferenceLeave(c.Request.Context(), ext), "left the conference")
}

type conferenceMuteBody struct {
	// Muted rather than a string state, so the caller cannot send something
	// the PBX would reject; the client maps it to v3's on/off.
	Muted bool `json:"muted"`
}

// POST /api/v1/secure/softphone/conference/mute
func (h *Handlers) ConferenceMute(c *gin.Context) {
	ext, ok := h.callerExtension(c)
	if !ok {
		return
	}
	var body conferenceMuteBody
	_ = c.ShouldBindJSON(&body)
	status := "unmuted in the conference"
	if body.Muted {
		status = "muted in the conference"
	}
	respondAMI(c, h.pbxV3.ConferenceMute(c.Request.Context(), ext, body.Muted), status)
}

// POST /api/v1/secure/softphone/whisper — supervisor only.
//
// Coaching: heard by the agent, not by the caller. A separate endpoint from
// monitor/barge because v3 gives it one — it is not a mode of
// /ami/supervision/start.
//
// NOT REACHABLE FROM THE UI TODAY, and the reason is worth knowing before
// wiring it up again. v3's /ami/whisper/start accepts only a target; it
// works out who is DOING the whispering from the token's own user, and
// this service authenticates as one shared account that owns no extension,
// so it answers "your account is not linked to a FreePBX extension"
// whoever clicked. The client therefore still uses the v2 spy action,
// which carries the supervisor's own channel in the request.
//
// Making this usable means giving v3 a per-user token rather than a
// service account — which the directory work makes possible, since agents
// are now v3 users, but which nothing here does yet.
func (h *Handlers) WhisperStart(c *gin.Context) {
	if h.pbxV3 == nil {
		apires.Error(c, http.StatusServiceUnavailable, "this needs pbx-worker v3 to be configured", nil)
		return
	}
	var body targetExtBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "target_ext is required", nil)
		return
	}
	respondAMI(c, h.pbxV3.WhisperStart(c.Request.Context(), body.TargetExt), "whisper started")
}

type superviseBody struct {
	TargetExt string `json:"target_ext" binding:"required"`
	// Mode is monitor or barge. Whisper has its own endpoint above.
	Mode string `json:"mode"`
}

// POST /api/v1/secure/softphone/supervise — supervisor only.
func (h *Handlers) SupervisionStart(c *gin.Context) {
	if h.pbxV3 == nil {
		apires.Error(c, http.StatusServiceUnavailable, "this needs pbx-worker v3 to be configured", nil)
		return
	}
	var body superviseBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "target_ext is required", nil)
		return
	}

	mode := pbxv3.SuperviseMonitor
	switch body.Mode {
	case "", "monitor":
		mode = pbxv3.SuperviseMonitor
	case "barge":
		mode = pbxv3.SuperviseBarge
	default:
		apires.Error(c, http.StatusUnprocessableEntity, "mode must be monitor or barge", nil)
		return
	}
	respondAMI(c, h.pbxV3.SupervisionStart(c.Request.Context(), mode, body.TargetExt), string(mode)+" started")
}

type attendedTransferBody struct {
	Extension string `json:"extension" binding:"required"`
}

// POST /api/v1/secure/softphone/transfer/attended
//
// Dials the target so the agent can speak to them with the caller on hold —
// the flow JsSipEngine.transferCall deliberately refuses to fake, since a
// REFER cannot express it.
func (h *Handlers) TransferAttended(c *gin.Context) {
	ext, ok := h.callerExtension(c)
	if !ok {
		return
	}
	var body attendedTransferBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "extension is required", nil)
		return
	}
	respondAMI(c, h.pbxV3.TransferAttended(c.Request.Context(), ext, body.Extension), "consulting")
}

// POST /api/v1/secure/softphone/transfer/attended/cancel — take the caller
// back rather than completing the handover.
func (h *Handlers) TransferAttendedCancel(c *gin.Context) {
	ext, ok := h.callerExtension(c)
	if !ok {
		return
	}
	respondAMI(c, h.pbxV3.TransferAttendedCancel(c.Request.Context(), ext), "transfer cancelled")
}
