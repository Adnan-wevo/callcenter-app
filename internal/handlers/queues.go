package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
)

// GET /api/v1/call-center/queues
//
// Sourced from pbx-worker's queue-names action — resolves open decision D8
// in docs/extraction-plan.md: Laravel's own PbxReportGateway::queueNames()
// is confirmed (both by reading the source and by a live call against the
// real staging worker) to be how queue names actually reach the UI, not a
// direct qstats read. qstats.ListQueueNames still exists
// (internal/db/qstats) for whatever future report genuinely needs the
// qname table, but this endpoint no longer uses it.
func (h *Handlers) ListQueues(c *gin.Context) {
	names, err := h.pbx.QueueNames(c.Request.Context())
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load queues", nil)
		return
	}
	apires.OK(c, names)
}

// GET /api/v1/call-center/agents
//
// Sourced from pbx-worker's agent-names action — same resolution as
// ListQueues above.
func (h *Handlers) ListAgents(c *gin.Context) {
	names, err := h.pbx.AgentNames(c.Request.Context())
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load agents", nil)
		return
	}
	apires.OK(c, names)
}
