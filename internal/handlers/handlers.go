// Package handlers wires HTTP requests to the gateway/db layers.
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/auth"
	"callcenter-service/internal/calllog"
	"callcenter-service/internal/db/qstats"
	"callcenter-service/internal/gateway/laravel"
	"callcenter-service/internal/gateway/pbxcontrol"
	"callcenter-service/internal/gateway/pbxworker"
	"callcenter-service/internal/reports"
	"callcenter-service/internal/softphone"
)

type Handlers struct {
	qstats          *qstats.Repository
	pbx             pbxworker.ReportsClient
	laravelCallback laravel.CallbackClient
	auth            *auth.Service
	softphone       *softphone.Service
	callLogs        *calllog.Repository
	pbxControl      *pbxcontrol.Client
	sipExtensions   *softphone.Repository
}

func New(
	qstatsRepo *qstats.Repository,
	pbx pbxworker.ReportsClient,
	laravelCallback laravel.CallbackClient,
	authSvc *auth.Service,
	softphoneSvc *softphone.Service,
	callLogs *calllog.Repository,
	pbxControl *pbxcontrol.Client,
	sipExtensions *softphone.Repository,
) *Handlers {
	return &Handlers{
		qstats:          qstatsRepo,
		pbx:             pbx,
		laravelCallback: laravelCallback,
		auth:            authSvc,
		softphone:       softphoneSvc,
		callLogs:        callLogs,
		pbxControl:      pbxControl,
		sipExtensions:   sipExtensions,
	}
}

// Healthz is unauthenticated, used for docker-compose/orchestrator health checks.
func (h *Handlers) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// parseListParams reads the filter query params shared by every report
// endpoint, matching what PbxReportGateway::buildParams() sends on to
// pbx-worker: a date range, a seconds-of-day sub-range (default: full day,
// 0-86399), and optional repeated queue/agent name filters.
//
// Two of Laravel's hidden rules are applied here (docs/extraction-plan.md
// §4.3):
//
//   - **Dates are parsed in the app timezone**, never UTC. They arrive naive
//     ("2026-09-20 00:00:00") and are re-emitted naive to pbx-worker; parsing
//     them as UTC would shift every day boundary by the Kuala Lumpur offset
//     and silently corrupt the hour/day-of-week groupings.
//   - **The range is clamped to 90 days**, silently, by moving the start
//     forward — the same thing enforceDateRangeLimit() does rather than
//     rejecting. The bool return says whether that happened, so the response
//     can tell the caller instead of just showing a different window than
//     they asked for.
//
// Defaults to today when no range is given.
func parseListParams(c *gin.Context) (pbxworker.ListParams, bool) {
	loc := reports.Location()
	now := time.Now().In(loc)
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	from := parseDateParam(c.Query("date_from"), startOfToday, loc)
	to := parseDateParam(c.Query("date_to"), now, loc)
	from, to, clamped := reports.ClampRange(from, to)

	p := pbxworker.ListParams{
		DateFrom:     from,
		DateTo:       to,
		SecondsStart: 0,
		SecondsEnd:   86399,
		Queues:       c.QueryArray("queues"),
		Agents:       c.QueryArray("agents"),
	}
	if v := c.Query("seconds_start"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 86399 {
			p.SecondsStart = n
		}
	}
	if v := c.Query("seconds_end"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 86399 {
			p.SecondsEnd = n
		}
	}

	return p, clamped
}

// Page sizes this service will honour, matching the client's length menu.
// An unrecognised per_page falls back to defaultPerPage rather than being
// obeyed: an unbounded page is how one request pulls a quarter of call
// records into memory.
const (
	defaultPerPage = 50
	maxPerPage     = 1000
)

// paginate slices a whole result set into one page and builds its counters.
//
// pbx-worker has no pagination of its own — every action returns the full
// window — so paging is this service's job (docs/extraction-plan.md §4.4).
// That also means `total` here is exact rather than an estimate: the whole
// set was already in hand.
func paginate[T any](c *gin.Context, rows []T) ([]T, apires.Meta) {
	perPage := defaultPerPage
	if v := c.Query("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxPerPage {
			perPage = n
		}
	}

	page := 1
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}

	total := len(rows)
	lastPage := (total + perPage - 1) / perPage
	if lastPage == 0 {
		lastPage = 1
	}

	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	// Never nil: `null` and `[]` mean different things to a client iterating
	// the value.
	out := make([]T, 0, end-start)
	out = append(out, rows[start:end]...)

	return out, apires.Meta{
		Page:     page,
		PerPage:  perPage,
		Total:    total,
		LastPage: lastPage,
	}
}

// parseDateParam accepts either a full naive datetime or a bare date, and
// interprets both in loc. A date alone means the start of that day.
func parseDateParam(v string, fallback time.Time, loc *time.Location) time.Time {
	if v == "" {
		return fallback
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc); err == nil {
		return t
	}
	if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
		return t
	}
	return fallback
}
