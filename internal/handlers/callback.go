package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/gateway/laravel"
)

// callbackRequestBody is the shape this service accepts from the frontend.
// TODO: confirm required fields once the real "click callback from
// unanswered-calls report" UI/flow is checked against the Laravel reference.
type callbackRequestBody struct {
	QueueID string  `json:"queue_id" binding:"required"`
	AgentID string  `json:"agent_id" binding:"required"`
	Note    *string `json:"note"`
}

// POST /api/v1/call-center/unanswered-calls/:id/callback
//
// Replaces the in-process Laravel call from CallCenter -> Callback module.
// This service can no longer call that code directly, so it makes an
// outbound signed HTTP call back to Laravel instead. See
// internal/gateway/laravel for the TODOs on exact endpoint/payload.
func (h *Handlers) RecordAdHocCallback(c *gin.Context) {
	unansweredCallID := c.Param("id")

	var body callbackRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "invalid request body", err.Error())
		return
	}

	err := h.laravelCallback.RecordAdHocCallback(c.Request.Context(), laravel.CallbackRequest{
		UnansweredCallID: unansweredCallID,
		QueueID:          body.QueueID,
		AgentID:          body.AgentID,
		AttemptedAt:      time.Now().UTC(),
		Note:             body.Note,
	})
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to record callback attempt in Laravel", nil)
		return
	}

	apires.Item(c, http.StatusAccepted, gin.H{"status": "recorded"})
}
