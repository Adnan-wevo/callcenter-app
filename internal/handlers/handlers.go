// Package handlers wires HTTP requests to the gateway/db layers.
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/db/qstats"
	"callcenter-service/internal/gateway/laravel"
	"callcenter-service/internal/gateway/pbxworker"
)

type Handlers struct {
	qstats          *qstats.Repository
	pbx             pbxworker.ReportsClient
	laravelCallback laravel.CallbackClient
}

func New(qstatsRepo *qstats.Repository, pbx pbxworker.ReportsClient, laravelCallback laravel.CallbackClient) *Handlers {
	return &Handlers{qstats: qstatsRepo, pbx: pbx, laravelCallback: laravelCallback}
}

// Healthz is unauthenticated, used for docker-compose/orchestrator health checks.
func (h *Handlers) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// parseListParams reads the filter/pagination query params assumed common
// to the list-style pbx-worker actions. See pbxworker.ListParams.
func parseListParams(c *gin.Context) pbxworker.ListParams {
	p := pbxworker.ListParams{
		QueueID: c.Query("queue_id"),
		AgentID: c.Query("agent_id"),
		Page:    1,
		PerPage: 25,
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			p.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			p.To = &t
		}
	}
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Page = n
		}
	}
	if v := c.Query("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.PerPage = n
		}
	}
	return p
}

func toAPIMeta(m pbxworker.Pagination) apires.Meta {
	return apires.Meta{
		CurrentPage: m.CurrentPage,
		PerPage:     m.PerPage,
		Total:       m.Total,
		LastPage:    m.LastPage,
	}
}
