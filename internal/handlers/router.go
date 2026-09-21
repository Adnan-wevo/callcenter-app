package handlers

import (
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/middleware"
	"callcenter-service/internal/security/hmacsig"
)

// NewRouter wires all routes. incomingCreds/tolerance/disabled configure the
// inbound HMAC middleware — see internal/config and README "Open questions"
// for the auth-scheme caveat.
func NewRouter(h *Handlers, incomingCreds hmacsig.Credentials, tolerance time.Duration, authDisabled bool) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", h.Healthz)

	v1 := r.Group("/api/v1/call-center")
	v1.Use(middleware.HMACVerify(incomingCreds, tolerance, authDisabled))
	{
		v1.GET("/queues", h.ListQueues)
		v1.GET("/agents", h.ListAgents)
		v1.GET("/answered-calls", h.ListAnsweredCalls)
		v1.GET("/unanswered-calls", h.ListUnansweredCalls)
		v1.GET("/agent-events", h.ListAgentEvents)
		v1.GET("/calls/search", h.SearchCalls)
		v1.GET("/calls/:id", h.GetCallDetail)
		v1.POST("/unanswered-calls/:id/callback", h.RecordAdHocCallback)
	}

	return r
}
