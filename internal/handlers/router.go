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

		// The softphone's own credentials. Gated by authentication alone:
		// every agent who may sign in may register a phone, and there is no
		// separate "may use the phone" permission in the catalogue.
		secure.GET("/softphone/bootstrap", h.SoftphoneBootstrap)

		sp := secure.Group("/softphone")
		{
			// The call-log write-path (Softphone::createCallLog and
			// friends). Gated by authentication alone, matching the PHP
			// original: any signed-in agent may record their own calls,
			// there is no separate permission for it there either.
			sp.POST("/call-logs", h.CreateCallLog)
			sp.GET("/call-logs/mine", h.ListMyCallLogs)
			sp.POST("/call-logs/:id/answered", h.MarkCallAnswered)
			sp.POST("/call-logs/:id/finalize", h.FinalizeCallLog)
			sp.POST("/detect-queue", h.DetectQueue)

			// Live PBX snapshots — reads, authentication only.
			sp.GET("/queues", h.LiveQueues)
			sp.GET("/agents", h.LiveAgents)
			sp.GET("/live-calls", h.LiveCalls)

			// Queue membership: an agent controls their OWN membership only
			// (see interfaceFor's own doc comment) — no extra permission
			// beyond authentication, matching QueueMemberController's real
			// route group in Modules/SoftPhone/routes/api.php.
			sp.POST("/queue/login", h.QueueLogin)
			sp.POST("/queue/logout", h.QueueLogout)
			sp.POST("/queue/pause", h.QueuePause)
			sp.POST("/queue/unpause", h.QueueUnpause)

			// Supervisor actions over OTHER agents' live calls — gated the
			// same way heal-crm gates them
			// (`can:softphone.supervise` in QueueActionController's route
			// group).
			supervise := middleware.RequirePermission(auth.PermSoftphoneSupervise)
			sp.POST("/queue/redirect", supervise, h.QueueRedirect)
			sp.POST("/queue/pickup", supervise, h.QueuePickup)
			sp.POST("/queue/spy", supervise, h.QueueSpy)
			sp.POST("/queue/agent-pause", supervise, h.QueueAgentPause)
			sp.POST("/queue/agent-unpause", supervise, h.QueueAgentUnpause)
			sp.POST("/queue/agent-logout", supervise, h.QueueAgentLogout)
		}

		// SIP extension administration — who may register as which
		// extension. Same four permission names as heal-crm's own
		// SipExtensions\Index Livewire component.
		admin := secure.Group("/admin/sip-extensions")
		{
			admin.GET("", middleware.RequirePermission(auth.PermSIPExtensionsIndex), h.ListSIPExtensions)
			admin.POST("", middleware.RequirePermission(auth.PermSIPExtensionsStore), h.CreateSIPExtension)
			admin.PUT("/:id", middleware.RequirePermission(auth.PermSIPExtensionsUpdate), h.UpdateSIPExtension)
			admin.DELETE("/:id", middleware.RequirePermission(auth.PermSIPExtensionsDestroy), h.DeleteSIPExtension)
		}

		// The user picker the SIP Extensions admin screen's create/edit form
		// needs ("assign to which user") — gated the same as reading the
		// extensions list itself, not a separate permission, since it exists
		// only to serve that one screen today.
		secure.GET("/admin/users", middleware.RequirePermission(auth.PermSIPExtensionsIndex), h.ListUsers)

		// User administration, backed by v3's directory — this service's own
		// store is seeded from environment variables and cannot grow an
		// account, which is why there was no way to add a user before.
		users := secure.Group("/admin/users")
		{
			users.GET("/manage", middleware.RequirePermission(auth.PermUsersIndex), h.ListManagedUsers)
			users.POST("", middleware.RequirePermission(auth.PermUsersStore), h.CreateUser)
			users.PUT("/:id", middleware.RequirePermission(auth.PermUsersUpdate), h.UpdateUser)
			users.DELETE("/:id", middleware.RequirePermission(auth.PermUsersDestroy), h.DeleteUser)
		}
		secure.GET("/admin/roles", middleware.RequirePermission(auth.PermUsersIndex), h.ListRoles)

		// Row-level security admin — same four permission names as
		// UserFilters\Index.php's own $this->authorize(...) calls.
		filters := secure.Group("/admin/user-filters")
		{
			filters.GET("", middleware.RequirePermission(auth.PermUserFiltersIndex), h.ListUserFilters)
			filters.PUT("/:id", middleware.RequirePermission(auth.PermUserFiltersUpdate), h.UpdateUserFilters)
			filters.DELETE("/:id", middleware.RequirePermission(auth.PermUserFiltersDestroy), h.ClearUserFilters)
		}

		settingsGroup := secure.Group("/call-center/settings")
		{
			settingsGroup.GET("/categories", middleware.RequirePermission(auth.PermSettingsIndex), h.ListSettingCategories)
			settingsGroup.GET("/:category", middleware.RequirePermission(auth.PermSettingsIndex), h.ListSettingsForCategory)
			settingsGroup.PUT("/:category", middleware.RequirePermission(auth.PermSettingsUpdate), h.UpdateSettingsForCategory)
		}

		queueGroupsGroup := secure.Group("/admin/queue-groups")
		{
			queueGroupsGroup.GET("", middleware.RequirePermission(auth.PermQueueGroupsIndex), h.ListQueueGroups)
			queueGroupsGroup.POST("", middleware.RequirePermission(auth.PermQueueGroupsStore), h.CreateQueueGroup)
			queueGroupsGroup.PUT("/:id", middleware.RequirePermission(auth.PermQueueGroupsUpdate), h.UpdateQueueGroup)
			queueGroupsGroup.DELETE("/:id", middleware.RequirePermission(auth.PermQueueGroupsDestroy), h.DeleteQueueGroup)
		}

		scheduledReportsGroup := secure.Group("/admin/scheduled-reports")
		{
			scheduledReportsGroup.GET("", middleware.RequirePermission(auth.PermScheduledReportsIndex), h.ListScheduledReports)
			scheduledReportsGroup.POST("", middleware.RequirePermission(auth.PermScheduledReportsStore), h.CreateScheduledReport)
			scheduledReportsGroup.PUT("/:id", middleware.RequirePermission(auth.PermScheduledReportsUpdate), h.UpdateScheduledReport)
			scheduledReportsGroup.DELETE("/:id", middleware.RequirePermission(auth.PermScheduledReportsDestroy), h.DeleteScheduledReport)
		}

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

			cc.GET("/distribution",
				middleware.RequirePermission(auth.PermDistribution), h.Distribution)

			cc.GET("/agent-performance",
				middleware.RequirePermission(auth.PermAgentPerformance), h.AgentPerformance)

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
