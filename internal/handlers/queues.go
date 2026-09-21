package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
)

// GET /api/v1/call-center/queues
//
// ASSUMPTION: sourced from qstats.queue_names directly (not the pbx-worker
// queue-names action) — see README "Open questions" for why this side was
// picked, and internal/gateway/pbxworker.HMACClient.QueueNames for the
// unused alternative implementation.
func (h *Handlers) ListQueues(c *gin.Context) {
	rows, err := h.qstats.ListQueueNames(c.Request.Context())
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load queues", nil)
		return
	}
	apires.OK(c, rows)
}

// GET /api/v1/call-center/agents
//
// ASSUMPTION: sourced from qstats.queue_agents directly — same rationale as
// ListQueues above. Optional ?queue_id= filter.
func (h *Handlers) ListAgents(c *gin.Context) {
	var queueID *int64
	if v := c.Query("queue_id"); v != "" {
		id, err := parseInt64(v)
		if err != nil {
			apires.Error(c, http.StatusBadRequest, "invalid queue_id", nil)
			return
		}
		queueID = &id
	}

	rows, err := h.qstats.ListQueueAgents(c.Request.Context(), queueID)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load agents", nil)
		return
	}
	apires.OK(c, rows)
}
