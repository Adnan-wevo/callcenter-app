package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/gateway/pbxworker"
)

// GET /api/v1/call-center/answered-calls
func (h *Handlers) ListAnsweredCalls(c *gin.Context) {
	p := parseListParams(c)
	rows, meta, err := h.pbx.AnsweredCalls(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load answered calls", nil)
		return
	}
	apires.Collection(c, http.StatusOK, rows, toAPIMeta(meta))
}

// GET /api/v1/call-center/unanswered-calls
func (h *Handlers) ListUnansweredCalls(c *gin.Context) {
	p := parseListParams(c)
	rows, meta, err := h.pbx.UnansweredCalls(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load unanswered calls", nil)
		return
	}
	apires.Collection(c, http.StatusOK, rows, toAPIMeta(meta))
}

// GET /api/v1/call-center/agent-events
func (h *Handlers) ListAgentEvents(c *gin.Context) {
	p := parseListParams(c)
	rows, meta, err := h.pbx.AgentEvents(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load agent events", nil)
		return
	}
	apires.Collection(c, http.StatusOK, rows, toAPIMeta(meta))
}

// GET /api/v1/call-center/calls/search
func (h *Handlers) SearchCalls(c *gin.Context) {
	p := pbxworker.CallSearchParams{
		ListParams: parseListParams(c),
		CallerID:   c.Query("caller_id"),
		Status:     c.Query("status"),
	}
	rows, meta, err := h.pbx.CallSearch(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to search calls", nil)
		return
	}
	apires.Collection(c, http.StatusOK, rows, toAPIMeta(meta))
}

// GET /api/v1/call-center/calls/:id
//
// TODO: recording_url currently comes straight from the pbx-worker response
// (see pbxworker.CallDetail). If the real API doesn't include it, enrich
// here via h.qstats.GetRecordingByCallID instead.
func (h *Handlers) GetCallDetail(c *gin.Context) {
	id := c.Param("id")
	detail, err := h.pbx.CallDetail(c.Request.Context(), id)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load call detail", nil)
		return
	}
	apires.OK(c, detail)
}
