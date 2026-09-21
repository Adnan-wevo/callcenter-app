// Package reports holds the report semantics that Laravel applies inside its
// query layer — the rules that change the NUMBERS and are invisible from an
// endpoint's shape.
//
// These are documented as §4.3 of docs/extraction-plan.md. They live in one
// package rather than inline in each handler because the failure mode is
// silent: a report that skips one of them returns plausible figures that
// quietly disagree with Laravel's, and nobody notices until an audit.
//
// # Where the values come from
//
// In Laravel these are rows in `call_center_settings`, read live by every
// report service and memoised per request. This service cannot read that
// table yet — whether it gets shared DB access or a small Laravel endpoint is
// open decision D3. Until that is settled, Defaults() returns the same
// fallbacks the PHP getters use, so behaviour matches an unconfigured
// install but NOT a configured one.
package reports

import "time"

// Settings are the three `call_center_settings` values the report services
// read. The defaults match the PHP side's own fallbacks.
type Settings struct {
	// SLAInterval is the answer-within threshold, in seconds, behind every
	// SLA percentage. Laravel: `sla.sla_interval`, default "20".
	SLAInterval int

	// ShortAbandonThreshold, in seconds, suppresses noise calls: an
	// unanswered call whose hold time is BELOW it is dropped entirely from
	// every summary, grouping, detail and export. Laravel:
	// `threshold.short_abandon_threshold`, default "5".
	ShortAbandonThreshold int

	// WrapUp, in seconds, is added to occupancy in the agent report.
	// Laravel: `threshold.wrap_up`, default "0".
	WrapUp int
}

// Defaults mirrors the PHP getters' fallback values.
func Defaults() Settings {
	return Settings{
		SLAInterval:           20,
		ShortAbandonThreshold: 5,
		WrapUp:                0,
	}
}

// AppTimezone is the timezone every report filter is expressed in.
//
// Laravel builds its date filters as NAIVE local strings ("2026-09-20
// 00:00:00") with no offset, parsed with Carbon against the app's configured
// timezone, and sends them to pbx-worker in that same naive form. The app is
// configured `Asia/Kuala_Lumpur`, not UTC.
//
// Defaulting to UTC anywhere in this service shifts every day boundary by
// eight hours and corrupts the day/hour/day-of-week groupings, so the zone is
// named here once and loaded through Location() rather than assumed.
const AppTimezone = "Asia/Kuala_Lumpur"

// MaxRangeDays is the window Laravel silently clamps any wider request down
// to (HasReportFilters::enforceDateRangeLimit). It does NOT reject — it pulls
// the start date forward — so a client asking for a year gets 90 days of
// figures rather than an error.
const MaxRangeDays = 90

// Location returns the app timezone, falling back to UTC only if the
// zoneinfo database is unavailable (a bare container without tzdata).
func Location() *time.Location {
	loc, err := time.LoadLocation(AppTimezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// ClampRange applies the 90-day limit the way Laravel does: silently, by
// moving `from` forward, and it also repairs an inverted range. The second
// return value reports whether anything was changed, so a caller can tell the
// user their window was narrowed instead of quietly showing different
// figures than they asked for.
func ClampRange(from, to time.Time) (time.Time, time.Time, bool) {
	clamped := false

	if to.Before(from) {
		to = from
		clamped = true
	}

	limit := to.AddDate(0, 0, -MaxRangeDays)
	if from.Before(limit) {
		from = limit
		clamped = true
	}

	return from, to, clamped
}
