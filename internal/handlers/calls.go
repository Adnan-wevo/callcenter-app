package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/gateway/pbxworker"
	"callcenter-service/internal/reports"
)

// GET /api/v1/secure/call-center/answered-calls
func (h *Handlers) ListAnsweredCalls(c *gin.Context) {
	p, _ := parseListParams(c)
	rows, err := h.pbx.AnsweredCalls(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load answered calls", nil)
		return
	}
	page, meta := paginate(c, rows)
	apires.Collection(c, http.StatusOK, page, meta)
}

// GET /api/v1/secure/call-center/unanswered-calls
//
// Short abandons are suppressed here, as Laravel does — see
// reports.FilterShortAbandons. Without it every abandonment figure this
// service reports would be higher than the monolith's for the same window.
func (h *Handlers) ListUnansweredCalls(c *gin.Context) {
	p, _ := parseListParams(c)
	rows, err := h.pbx.UnansweredCalls(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load unanswered calls", nil)
		return
	}
	kept, _ := reports.FilterShortAbandons(rows, reports.Defaults().ShortAbandonThreshold)
	page, meta := paginate(c, kept)
	apires.Collection(c, http.StatusOK, page, meta)
}

// GET /api/v1/secure/call-center/agent-events
func (h *Handlers) ListAgentEvents(c *gin.Context) {
	p, _ := parseListParams(c)
	rows, err := h.pbx.AgentEvents(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load agent events", nil)
		return
	}
	page, meta := paginate(c, rows)
	apires.Collection(c, http.StatusOK, page, meta)
}

// GET /api/v1/secure/call-center/calls/search
func (h *Handlers) SearchCalls(c *gin.Context) {
	base, _ := parseListParams(c)
	p := pbxworker.CallSearchParams{
		ListParams: base,
		CallerID:   c.Query("caller_id"),
		UniqueID:   c.Query("unique_id"),
	}
	// duration_operator/duration_seconds are only meaningful together,
	// matching PbxReportGateway.php.
	if op := c.Query("duration_operator"); op != "" {
		if v := c.Query("duration_seconds"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				p.DurationOperator = op
				p.DurationSeconds = &n
			}
		}
	}

	rows, err := h.pbx.CallSearch(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to search calls", nil)
		return
	}
	page, meta := paginate(c, rows)
	apires.Collection(c, http.StatusOK, page, meta)
}

// GET /api/v1/secure/call-center/calls/:id
//
// :id is the call's uniqueid. The response is the call's full timeline — an
// array of rows, not a single summary object — so it is returned whole
// rather than paginated.
func (h *Handlers) GetCallDetail(c *gin.Context) {
	rows, err := h.pbx.CallDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load call detail", nil)
		return
	}
	apires.OK(c, rows)
}
