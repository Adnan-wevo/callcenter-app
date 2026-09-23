package pbxworker

import (
	"context"
	"fmt"
	"net/http"
)

// JWTClient is a placeholder for pbx-worker v3 (Go, JWT auth). It satisfies
// ReportsClient so the rest of the codebase can already depend on the
// interface and swap PBX_WORKER_MODE=jwt in later without any caller
// changes — but none of the methods are implemented yet, and finishing it
// is NOT the small job the stub's original TODO implied.
//
// Checked against the v3 source (reference/wevetel-go-v3) 2026-09-23. Two
// findings, both blocking:
//
//  1. Nothing is deployed to talk to. v2 answers on
//     sbc.wevetel.com/wevetel-pbx-worker/api/reports.php (401, demanding
//     the HMAC headers HMACClient sends); every v3 path there 404s.
//     Standing v3 up is its own service — own MySQL schema (pbx_api),
//     migrations, JWT secret, config/freepbx_connections.yaml.
//  2. v3 is not shaped like this interface. Routes are
//     /{APP_SYSPREFIX}/api/v1/secure/callcenter/... and v3 aggregates
//     SERVER-side, where this service pulls raw rows and aggregates them
//     itself in internal/reports. Only four of the seven methods below
//     have a clean counterpart:
//
//     QueueNames  -> GET /lookups/queues
//     AgentNames  -> GET /lookups/agents
//     CallSearch  -> GET /search/datatables
//     CallDetail  -> GET /search/call/:uniqueid
//
//     AnsweredCalls/UnansweredCalls/AgentEvents have none: v3 exposes
//     finished reports instead (/answered/summary, /answered/by-queue,
//     /distribution/by-hour, /agent/availability, ...), with row-level
//     data only via the various /detail/datatables endpoints. Backing
//     those three methods by re-aggregating /detail/datatables would mean
//     redoing work v3 already did.
//
// So adopting v3 is a handler-layer decision (call its report endpoints
// directly and drop most of internal/reports), not a gateway swap behind
// this interface. Decision 2026-09-23: stay on v2 until v3 is deployed.
// Auth, when it is: POST /{prefix}/api/v1/open/auth/login returns a 60min
// access token plus a refresh token, permissions baked into the claims.
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

func (c *JWTClient) QueueNames(ctx context.Context) ([]string, error) { return nil, errNotImplemented }
func (c *JWTClient) AgentNames(ctx context.Context) ([]string, error) { return nil, errNotImplemented }
func (c *JWTClient) AnsweredCalls(ctx context.Context, p ListParams) ([]AnsweredCall, error) {
	return nil, errNotImplemented
}
func (c *JWTClient) UnansweredCalls(ctx context.Context, p ListParams) ([]UnansweredCall, error) {
	return nil, errNotImplemented
}
func (c *JWTClient) AgentEvents(ctx context.Context, p ListParams) ([]AgentEvent, error) {
	return nil, errNotImplemented
}
func (c *JWTClient) CallSearch(ctx context.Context, p CallSearchParams) ([]CallSearchResult, error) {
	return nil, errNotImplemented
}
func (c *JWTClient) CallDetail(ctx context.Context, uniqueID string) ([]CallDetailRow, error) {
	return nil, errNotImplemented
}
