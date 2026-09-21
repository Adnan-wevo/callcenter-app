package pbxworker

import "context"

// ReportsClient is implemented once per pbx-worker version so the rest of
// the service (handlers) can stay agnostic to which auth scheme / API shape
// is behind it. See hmac_client.go (v2, live today, confirmed against
// Modules/CallCenter/app/Services/Pbx/PbxReportGateway.php) and
// jwt_client.go (v3, TODO — not migrated yet). None of these actions have
// pagination on the real API — see hmac_client.go's apiEnvelope.
type ReportsClient interface {
	QueueNames(ctx context.Context) ([]string, error)
	AgentNames(ctx context.Context) ([]string, error)
	AnsweredCalls(ctx context.Context, p ListParams) ([]AnsweredCall, error)
	UnansweredCalls(ctx context.Context, p ListParams) ([]UnansweredCall, error)
	AgentEvents(ctx context.Context, p ListParams) ([]AgentEvent, error)
	CallSearch(ctx context.Context, p CallSearchParams) ([]CallSearchResult, error)
	CallDetail(ctx context.Context, uniqueID string) ([]CallDetailRow, error)
}
