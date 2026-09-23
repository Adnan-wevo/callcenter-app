package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/reports"
)

// distributionRow is one queue's line, extending dashboardSummary's
// queueRow with the per-queue SLA% and average hold DistributionReportService
// offers on the real Laravel side that the dashboard's own summary does not
// need.
type distributionRow struct {
	QueueName          string  `json:"queue_name"`
	Received           int     `json:"received"`
	Answered           int     `json:"answered"`
	Unanswered         int     `json:"unanswered"`
	SLAPercent         float64 `json:"sla_percent"`
	AvgHoldSeconds     int     `json:"avg_hold_seconds"`
	AvgDurationSeconds int     `json:"avg_duration_seconds"`
}

type distributionResponse struct {
	SLAInterval           int               `json:"sla_interval"`
	ShortAbandonsExcluded int               `json:"short_abandons_excluded"`
	RangeClamped          bool              `json:"range_clamped"`
	Queues                []distributionRow `json:"queues"`
}

// GET /api/v1/secure/call-center/distribution
//
// Per-queue received/answered/unanswered/SLA — DistributionReportService's
// shape. Built from the same two gateway calls DashboardSummary uses
// (answered-calls, unanswered-calls over the window) rather than a third
// pbx-worker action, since pbx-worker has no distribution-specific
// endpoint of its own — Laravel's DistributionReportService computes this
// from the identical two report calls.
func (h *Handlers) Distribution(c *gin.Context) {
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

	type acc struct {
		received, answeredN, unansweredN, withinSLA, holdTotal, durationTotal int
	}
	byQueue := map[string]*acc{}
	order := []string{}
	get := func(name string) *acc {
		a, ok := byQueue[name]
		if !ok {
			a = &acc{}
			byQueue[name] = a
			order = append(order, name)
		}
		return a
	}

	for _, call := range answered {
		a := get(call.QueueName)
		a.received++
		a.answeredN++
		a.holdTotal += call.HoldTime.Int()
		a.durationTotal += call.Duration.Int()
		if call.RingTime.Int() <= settings.SLAInterval {
			a.withinSLA++
		}
	}
	for _, call := range kept {
		a := get(call.QueueName)
		a.received++
		a.unansweredN++
	}

	rows := make([]distributionRow, 0, len(order))
	for _, name := range order {
		a := byQueue[name]
		row := distributionRow{
			QueueName:  name,
			Received:   a.received,
			Answered:   a.answeredN,
			Unanswered: a.unansweredN,
		}
		if a.answeredN > 0 {
			row.SLAPercent = float64(a.withinSLA) / float64(a.answeredN) * 100
			row.AvgHoldSeconds = a.holdTotal / a.answeredN
			row.AvgDurationSeconds = a.durationTotal / a.answeredN
		}
		rows = append(rows, row)
	}

	apires.OK(c, distributionResponse{
		SLAInterval:           settings.SLAInterval,
		ShortAbandonsExcluded: excluded,
		RangeClamped:          clamped,
		Queues:                rows,
	})
}
