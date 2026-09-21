package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
)

// GET /api/v1/call-center/queues
//
// Sourced from qstats.qname directly. Whether this should instead go
// through pbxworker.ReportsClient.QueueNames (matching how Laravel
// actually sources queue names — see PbxReportGateway.php) is an open
// decision, not resolved by this fix; see README.
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
// Sourced from qstats.qagent directly — same open-decision caveat as
// ListQueues above. The real qagent table has no queue-linkage column, so
// there is no per-queue filter here (an earlier version of this handler
// assumed a queue_id filter the schema doesn't support).
func (h *Handlers) ListAgents(c *gin.Context) {
	rows, err := h.qstats.ListQueueAgents(c.Request.Context())
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load agents", nil)
		return
	}
	apires.OK(c, rows)
}
