package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/reports"
)

// queueRow is one queue's line in the distribution block.
type queueRow struct {
	QueueName  string `json:"queue_name"`
	Answered   int    `json:"answered"`
	Unanswered int    `json:"unanswered"`
	Received   int    `json:"received"`
}

type dashboardSummary struct {
	Answered   int `json:"answered"`
	Unanswered int `json:"unanswered"`
	Received   int `json:"received"`

	AvgHoldSeconds     int `json:"avg_hold_seconds"`
	AvgDurationSeconds int `json:"avg_duration_seconds"`

	// SLAPercent is the share of answered calls whose ring time was within
	// the configured SLA interval.
	SLAPercent  float64 `json:"sla_percent"`
	SLAInterval int     `json:"sla_interval"`

	// ShortAbandonsExcluded counts unanswered calls suppressed by the
	// short-abandon threshold. Laravel drops them silently; reporting the
	// number makes the rule visible instead of leaving a figure that looks
	// wrong to anyone comparing against a raw switch report.
	ShortAbandonsExcluded int `json:"short_abandons_excluded"`

	// RangeClamped is true when the requested window exceeded the 90-day
	// limit and was narrowed.
	RangeClamped bool `json:"range_clamped"`

	Distribution []queueRow `json:"distribution"`
}

// GET /api/v1/secure/call-center/dashboard/summary
//
// Mirrors DashboardSummaryService::summary(): two gateway calls (answered and
// unanswered over the same window), folded into totals plus a per-queue
// breakdown in a single pass.
func (h *Handlers) DashboardSummary(c *gin.Context) {
	p, clamped := parseListParams(c)
	settings := h.loadReportSettings(c.Request.Context())

	answered, err := h.pbx.AnsweredCalls(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load answered calls", nil)
		return
	}
	unanswered, err := h.pbx.UnansweredCalls(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load unanswered calls", nil)
		return
	}

	kept, excluded := reports.FilterShortAbandons(unanswered, settings.ShortAbandonThreshold)

	out := dashboardSummary{
		Answered:              len(answered),
		Unanswered:            len(kept),
		Received:              len(answered) + len(kept),
		SLAInterval:           settings.SLAInterval,
		ShortAbandonsExcluded: excluded,
		RangeClamped:          clamped,
		Distribution:          []queueRow{},
	}

	var holdTotal, durationTotal, withinSLA int

	// Index holds POSITIONS, not pointers: appending to Distribution below can
	// reallocate its backing array, which would leave any retained pointer
	// writing into a stale copy.
	position := map[string]int{}
	queueAt := func(name string) *queueRow {
		i, ok := position[name]
		if !ok {
			out.Distribution = append(out.Distribution, queueRow{QueueName: name})
			i = len(out.Distribution) - 1
			position[name] = i
		}
		return &out.Distribution[i]
	}

	for _, call := range answered {
		holdTotal += call.HoldTime.Int()
		durationTotal += call.Duration.Int()
		if call.RingTime.Int() <= settings.SLAInterval {
			withinSLA++
		}
		row := queueAt(call.QueueName)
		row.Answered++
		row.Received++
	}
	for _, call := range kept {
		row := queueAt(call.QueueName)
		row.Unanswered++
		row.Received++
	}

	if n := len(answered); n > 0 {
		out.AvgHoldSeconds = holdTotal / n
		out.AvgDurationSeconds = durationTotal / n
		out.SLAPercent = float64(withinSLA) / float64(n) * 100
	}

	apires.OK(c, out)
}
