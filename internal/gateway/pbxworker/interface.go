package pbxworker

import "context"

// ReportsClient is implemented once per pbx-worker version so the rest of
// the service (handlers) can stay agnostic to which auth scheme / API shape
// is behind it. See hmac_client.go (v2, live today) and jwt_client.go
// (v3, TODO — not migrated yet).
type ReportsClient interface {
	QueueNames(ctx context.Context) ([]QueueName, error)
	AgentNames(ctx context.Context) ([]AgentName, error)
	AnsweredCalls(ctx context.Context, p ListParams) ([]AnsweredCall, Pagination, error)
	UnansweredCalls(ctx context.Context, p ListParams) ([]UnansweredCall, Pagination, error)
	AgentEvents(ctx context.Context, p ListParams) ([]AgentEvent, Pagination, error)
	CallSearch(ctx context.Context, p CallSearchParams) ([]CallSummary, Pagination, error)
	CallDetail(ctx context.Context, callID string) (*CallDetail, error)
}
