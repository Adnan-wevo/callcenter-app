package pbxworker

import (
	"context"
	"fmt"
	"net/http"
)

// JWTClient is a placeholder for pbx-worker v3 (Go, JWT auth, /wmpbxworker
// prefix). It satisfies ReportsClient so the rest of the codebase can
// already depend on the interface and swap PBX_WORKER_MODE=jwt in later
// without any caller changes — but none of the methods are implemented yet.
//
// TODO: implement once v3's actual endpoint list, request/response shapes,
// and JWT acquisition/refresh flow are confirmed. Until then this exists
// only to prove the interface abstraction holds.
type JWTClient struct {
	http    *http.Client
	baseURL string
	token   string
}

func NewJWTClient(httpClient *http.Client, baseURL, token string) *JWTClient {
	return &JWTClient{http: httpClient, baseURL: baseURL, token: token}
}

var _ ReportsClient = (*JWTClient)(nil)

var errNotImplemented = fmt.Errorf("pbxworker: v3 (JWT) client not implemented yet — TODO confirm /wmpbxworker endpoints against reference code")

func (c *JWTClient) QueueNames(ctx context.Context) ([]QueueName, error) { return nil, errNotImplemented }
func (c *JWTClient) AgentNames(ctx context.Context) ([]AgentName, error) { return nil, errNotImplemented }
func (c *JWTClient) AnsweredCalls(ctx context.Context, p ListParams) ([]AnsweredCall, Pagination, error) {
	return nil, Pagination{}, errNotImplemented
}
func (c *JWTClient) UnansweredCalls(ctx context.Context, p ListParams) ([]UnansweredCall, Pagination, error) {
	return nil, Pagination{}, errNotImplemented
}
func (c *JWTClient) AgentEvents(ctx context.Context, p ListParams) ([]AgentEvent, Pagination, error) {
	return nil, Pagination{}, errNotImplemented
}
func (c *JWTClient) CallSearch(ctx context.Context, p CallSearchParams) ([]CallSummary, Pagination, error) {
	return nil, Pagination{}, errNotImplemented
}
func (c *JWTClient) CallDetail(ctx context.Context, callID string) (*CallDetail, error) {
	return nil, errNotImplemented
}
