package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/gateway/pbxworker"
	"callcenter-service/internal/reports"
)

// agentStats is one agent's row in the response. Session/pause seconds come
// from replaying agent-events in order (see pairEvents below); calls
// handled and talk/hold averages come from answered-calls grouped by agent
// — that data is already reliable per-call, so it is not re-derived from
// raw events the way session/pause time has to be.
type agentStats struct {
	AgentName        string  `json:"agent_name"`
	CallsHandled     int     `json:"calls_handled"`
	AvgTalkSeconds   int     `json:"avg_talk_seconds"`
	AvgHoldSeconds   int     `json:"avg_hold_seconds"`
	SessionSeconds   int     `json:"session_seconds"`
	PauseSeconds     int     `json:"pause_seconds"`
	WrapUpSeconds    int     `json:"wrap_up_seconds"`
	OccupancyPercent float64 `json:"occupancy_percent"`
}

// GET /api/v1/secure/call-center/agent-performance
//
// # Ported with a caveat, not confirmed against Laravel's exact algorithm
//
// AgentReportService::summary() on the real Laravel side replays the raw
// agent-event stream and pairs open/close events per agent — this endpoint
// does the same shape of work (see pairEvents), but the EXACT event-name
// vocabulary it pairs on (AGENTLOGIN/AGENTLOGOFF, PAUSE/UNPAUSE) is the
// standard Asterisk queue_log vocabulary, not a value confirmed against
// Laravel's own source the way e.g. the unanswered-call event enum
// (ABANDON/EXITWITHTIMEOUT/...) was. If the real pbx-worker uses different
// event strings for these, session/pause seconds below will silently read
// as zero rather than fail loudly — that asymmetry is worth fixing before
// trusting this screen's numbers over a raw switch report.
func (h *Handlers) AgentPerformance(c *gin.Context) {
	p, clamped := parseListParams(c)
	settings := reports.Defaults()

	events, err := h.pbx.AgentEvents(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load agent events", nil)
		return
	}
	answered, err := h.pbx.AnsweredCalls(c.Request.Context(), p)
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "failed to load answered calls", nil)
		return
	}

	sessionSecs, pauseSecs := pairEvents(events)

	type talkAcc struct {
		calls, talkTotal, holdTotal int
	}
	byAgent := map[string]*talkAcc{}
	order := []string{}
	get := func(name string) *talkAcc {
		a, ok := byAgent[name]
		if !ok {
			a = &talkAcc{}
			byAgent[name] = a
			order = append(order, name)
		}
		return a
	}
	for _, call := range answered {
		if call.AgentName == "" {
			continue
		}
		a := get(call.AgentName)
		a.calls++
		a.talkTotal += call.Duration.Int()
		a.holdTotal += call.HoldTime.Int()
	}
	// Include agents who logged in/paused but took no calls in the window —
	// an occupancy of 0% is itself the finding, not a row to hide.
	for name := range sessionSecs {
		get(name)
	}
	for name := range pauseSecs {
		get(name)
	}

	rows := make([]agentStats, 0, len(order))
	for _, name := range order {
		a := byAgent[name]
		row := agentStats{
			AgentName:      name,
			CallsHandled:   a.calls,
			SessionSeconds: sessionSecs[name],
			PauseSeconds:   pauseSecs[name],
			WrapUpSeconds:  a.calls * settings.WrapUp,
		}
		if a.calls > 0 {
			row.AvgTalkSeconds = a.talkTotal / a.calls
			row.AvgHoldSeconds = a.holdTotal / a.calls
		}
		if row.SessionSeconds > 0 {
			busy := a.talkTotal + row.WrapUpSeconds
			row.OccupancyPercent = float64(busy) / float64(row.SessionSeconds) * 100
		}
		rows = append(rows, row)
	}

	apires.Item(c, http.StatusOK, gin.H{
		"agents":        rows,
		"range_clamped": clamped,
	})
}

// pairEvents replays the event stream IN DATETIME ORDER and pairs
// login/logout and pause/unpause per agent, summing durations across
// however many sessions/pauses fall inside the window. An open session at
// the END of the window (login with no matching logoff yet — the agent is
// still logged in when the window closes) counts up to the window's own
// end, mirroring how a live report would treat "still ongoing" rather than
// discarding it.
func pairEvents(events []pbxworker.AgentEvent) (sessionSeconds, pauseSeconds map[string]int) {
	sessionSeconds = map[string]int{}
	pauseSeconds = map[string]int{}

	type row struct {
		when  time.Time
		agent string
		event string
	}
	rows := make([]row, 0, len(events))
	var windowEnd time.Time
	for _, e := range events {
		t, err := time.Parse("2006-01-02 15:04:05", e.Datetime)
		if err != nil {
			continue // an unparsable timestamp cannot be ordered; skip rather than guess.
		}
		if t.After(windowEnd) {
			windowEnd = t
		}
		rows = append(rows, row{when: t, agent: e.AgentName, event: e.Event})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].when.Before(rows[j].when) })

	openLogin := map[string]time.Time{}
	openPause := map[string]time.Time{}

	for _, r := range rows {
		switch r.event {
		case "AGENTLOGIN":
			openLogin[r.agent] = r.when
		case "AGENTLOGOFF":
			if start, ok := openLogin[r.agent]; ok {
				sessionSeconds[r.agent] += int(r.when.Sub(start).Seconds())
				delete(openLogin, r.agent)
			}
		case "PAUSE", "PAUSEALL":
			openPause[r.agent] = r.when
		case "UNPAUSE", "UNPAUSEALL":
			if start, ok := openPause[r.agent]; ok {
				pauseSeconds[r.agent] += int(r.when.Sub(start).Seconds())
				delete(openPause, r.agent)
			}
		}
	}
	// Close out sessions/pauses still open at the end of the window.
	for agent, start := range openLogin {
		sessionSeconds[agent] += int(windowEnd.Sub(start).Seconds())
	}
	for agent, start := range openPause {
		pauseSeconds[agent] += int(windowEnd.Sub(start).Seconds())
	}

	return sessionSeconds, pauseSeconds
}
