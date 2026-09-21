package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/auth"
	"callcenter-service/internal/middleware"
)

// NewRouter wires every route.
//
// # Path shape
//
// `/api/v1/{open|secure}/...` — `open` needs no credential, `secure` needs a
// bearer token. The split is in the URL rather than only in the middleware
// stack so a log line or a bookmark says which it was.
//
// # Two different auth schemes, deliberately
//
//   - **Browser traffic** (everything under `secure`) uses the JWT this
//     service issues. A browser cannot hold the HMAC shared secret.
//   - **Service-to-service** traffic (this service -> pbx-worker, this
//     service -> Laravel) stays HMAC-signed; see internal/security/hmacsig.
//     Nothing inbound uses HMAC today — internal/middleware.HMACVerify is
//     kept for when Laravel needs to call in.
//
// # Every secure route names its own permission
//
// There is no permissive default: a route with no RequirePermission is one
// any signed-in caller may use, and that is only true of the authority read
// itself (which must never 403 — the client resyncs through it after a
// forbidden response, and a 403 there would loop).
func NewRouter(h *Handlers, authSvc *auth.Service, corsOrigins []string) *gin.Engine {
	r := gin.Default()
	r.Use(CORS(corsOrigins))

	r.GET("/healthz", h.Healthz)

	v1 := r.Group("/api/v1")

	open := v1.Group("/open")
	{
		open.POST("/auth/login", h.Login)
	}

	secure := v1.Group("/secure")
	secure.Use(middleware.JWTAuth(authSvc))
	{
		secure.GET("/me/authority", h.MyAuthority)

		cc := secure.Group("/call-center")
		{
			// Lookups feed the filter bar on every report screen, so they are
			// admitted by holding ANY report permission rather than by naming
			// one screen's.
			anyReport := middleware.RequirePermission(
				auth.PermDashboard,
				auth.PermAnsweredIndex,
				auth.PermUnansweredIndex,
				auth.PermCallSearchIndex,
				auth.PermAgentPerformance,
				auth.PermDistribution,
			)
			cc.GET("/queues", anyReport, h.ListQueues)
			cc.GET("/agents", anyReport, h.ListAgents)

			cc.GET("/dashboard/summary",
				middleware.RequirePermission(auth.PermDashboard), h.DashboardSummary)

			cc.GET("/answered-calls",
				middleware.RequirePermission(auth.PermAnsweredIndex), h.ListAnsweredCalls)

			cc.GET("/unanswered-calls",
				middleware.RequirePermission(auth.PermUnansweredIndex), h.ListUnansweredCalls)

			cc.GET("/agent-events",
				middleware.RequirePermission(auth.PermAgentPerformance), h.ListAgentEvents)

			cc.GET("/calls/search",
				middleware.RequirePermission(auth.PermCallSearchIndex), h.SearchCalls)

			// :id is the call's uniqueid — pbx-worker's call-detail action keys
			// on call_uniqueid, not a numeric id.
			cc.GET("/calls/:id",
				middleware.RequirePermission(auth.PermCallSearchIndex), h.GetCallDetail)

			// The callback attempt. The acting agent comes from the TOKEN, never
			// from the request body — see handlers/callback.go.
			cc.POST("/unanswered-calls/callback",
				middleware.RequirePermission(auth.PermUnansweredCallback), h.RecordAdHocCallback)
		}
	}

	return r
}

// CORS allows the Angular dev server (a different origin) to call this
// service. Origins are an explicit allow-list from configuration: a
// reflected-origin wildcard combined with credentials is how a browser ends
// up sending a token to whoever asked.
func CORS(allowed []string) gin.HandlerFunc {
	allowSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		if o = strings.TrimSpace(o); o != "" {
			allowSet[o] = true
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowSet[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
