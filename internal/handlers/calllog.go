package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/calllog"
	"callcenter-service/internal/middleware"
)

// createCallLogBody mirrors Softphone::createCallLog()'s $data array
// field-for-field.
type createCallLogBody struct {
	CallerID    string `json:"caller_id"`
	CallerName  string `json:"caller_name"`
	Queue       string `json:"queue"`
	Direction   string `json:"direction"`
	Channel     string `json:"channel"`
	UniqueID    string `json:"unique_id"`
	Destination string `json:"destination"`
}

// POST /api/v1/secure/softphone/call-logs
//
// The browser calls this the moment a call starts ringing (inbound) or is
// dialled (outbound) — the direct equivalent of Softphone::createCallLog(),
// called from the Alpine JS side of heal-crm's softphone panel. See
// internal/calllog's own doc comment for why this reconciles against an
// existing row rather than always inserting.
func (h *Handlers) CreateCallLog(c *gin.Context) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	var body createCallLogBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "invalid request body", nil)
		return
	}

	extension := ""
	if ext, ok := h.softphone.ExtensionFor(c.Request.Context(), user.ID); ok {
		extension = ext
	}

	id, err := h.callLogs.CreateOrMerge(c.Request.Context(), calllog.CreateInput{
		CallerID:    body.CallerID,
		CallerName:  body.CallerName,
		Queue:       body.Queue,
		Direction:   body.Direction,
		Channel:     body.Channel,
		UniqueID:    body.UniqueID,
		Destination: body.Destination,
		UserID:      user.ID,
		Extension:   extension,
	})
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not record the call", nil)
		return
	}

	apires.Item(c, http.StatusCreated, gin.H{"id": id})
}

// POST /api/v1/secure/softphone/call-logs/:id/answered
//
// Softphone::markCallAnswered() ported directly.
func (h *Handlers) MarkCallAnswered(c *gin.Context) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	extension := ""
	if ext, ok := h.softphone.ExtensionFor(c.Request.Context(), user.ID); ok {
		extension = ext
	}

	if err := h.callLogs.MarkAnswered(c.Request.Context(), c.Param("id"), extension, user.ID); err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not update the call", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "answered"})
}

type finalizeCallLogBody struct {
	Status      string `json:"status"`
	Duration    int    `json:"duration"`
	WaitSeconds int    `json:"wait_seconds"`
	Delete      bool   `json:"_delete"`
}

// POST /api/v1/secure/softphone/call-logs/:id/finalize
//
// Softphone::finalizeCallLog() ported directly, including the guard that
// never lets an answered call be downgraded to missed/abandoned.
func (h *Handlers) FinalizeCallLog(c *gin.Context) {
	var body finalizeCallLogBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "invalid request body", nil)
		return
	}

	err := h.callLogs.Finalize(c.Request.Context(), c.Param("id"), calllog.FinalizeInput{
		Status:      body.Status,
		Duration:    body.Duration,
		WaitSeconds: body.WaitSeconds,
		Delete:      body.Delete,
	})
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not finalize the call", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "finalized"})
}

type detectQueueBody struct {
	CallerID  string `json:"caller_id" binding:"required"`
	CallLogID string `json:"call_log_id"`
	UniqueID  string `json:"unique_id"`
	Channel   string `json:"channel"`
}

// POST /api/v1/secure/softphone/detect-queue
//
// Softphone::detectQueue()'s DB-backed layers, plus a live PBX-worker
// snapshot lookup — see calllog.Repository.DetectQueue's own doc comment
// for exactly which of the PHP original's 5 layers are and are not here.
func (h *Handlers) DetectQueue(c *gin.Context) {
	var body detectQueueBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "caller_id is required", nil)
		return
	}

	var callLogID, uniqueID, channel *string
	if body.CallLogID != "" {
		callLogID = &body.CallLogID
	}
	if body.UniqueID != "" {
		uniqueID = &body.UniqueID
	}
	if body.Channel != "" {
		channel = &body.Channel
	}

	liveLookup := func(callerID string) (string, bool) {
		if h.pbxControl == nil {
			return "", false
		}
		snap, _, err := h.pbxControl.Queues(c.Request.Context())
		if err != nil {
			return "", false
		}
		for queueID, q := range snap.Queues {
			for _, caller := range q.Callers {
				if caller.CallerID.String() == callerID {
					return queueID, true
				}
			}
		}
		return "", false
	}

	result, err := h.callLogs.DetectQueue(c.Request.Context(), body.CallerID, callLogID, uniqueID, channel, liveLookup)
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not detect queue", nil)
		return
	}

	apires.Item(c, http.StatusOK, gin.H{
		"queue":       result.Queue,
		"call_log_id": result.CallLogID,
	})
}
