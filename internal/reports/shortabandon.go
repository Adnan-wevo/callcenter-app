package reports

import "callcenter-service/internal/gateway/pbxworker"

// FilterShortAbandons drops unanswered calls whose hold time is BELOW the
// threshold, returning the survivors and how many were removed.
//
// This mirrors UnansweredCallReportService / DistributionReportService /
// DashboardSummaryService on the Laravel side, all of which suppress these
// rows from every summary, grouping, detail and export. A caller who hangs up
// after two seconds is a misdial, not an abandoned call, and counting them
// makes every abandonment figure and every SLA percentage wrong.
//
// The comparison is strictly-less-than, matching the PHP: a call held for
// exactly the threshold is KEPT.
//
// A threshold of zero or less disables the rule entirely and keeps every row.
func FilterShortAbandons(calls []pbxworker.UnansweredCall, thresholdSeconds int) (kept []pbxworker.UnansweredCall, excluded int) {
	if thresholdSeconds <= 0 {
		return calls, 0
	}

	kept = make([]pbxworker.UnansweredCall, 0, len(calls))
	for _, call := range calls {
		if call.HoldTime < thresholdSeconds {
			excluded++
			continue
		}
		kept = append(kept, call)
	}

	return kept, excluded
}
