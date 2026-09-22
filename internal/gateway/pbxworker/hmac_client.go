package pbxworker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"callcenter-service/internal/security/hmacsig"
)

// HMACClient talks to pbx-worker v2 (PHP). CONFIRMED against
// Modules/CallCenter/app/Services/Pbx/PbxReportGateway.php: single endpoint
// GET /api/reports.php?action=<name>, always HMAC-signed (no per-action
// auth switching, no JWT anywhere in this gateway), response envelope is
// {"status": "ok"|..., "data": ..., "message"?: ...} with NO
// pagination/meta on any action.
type HMACClient struct {
	http    *http.Client
	baseURL string
	// basePath is the path component of baseURL, e.g. "/wevetel-pbx-worker"
	// when baseURL is "https://sbc.wevetel.com/wevetel-pbx-worker". It must
	// be prepended to every signed URI (see (c *HMACClient) get) — confirmed
	// live against the real staging worker, which signs against
	// $_SERVER['REQUEST_URI'] (PbxWorkerGateway.php's own basePath + uri).
	// Signing only "/api/reports.php?..." works against a bare-root
	// deployment (mock-pbx-worker has no path prefix, which is why this went
	// unnoticed locally) and fails signature verification the moment
	// PBX_WORKER_BASE_URL carries a path, which the real deployment's does.
	basePath string
	creds    hmacsig.Credentials
}

func NewHMACClient(httpClient *http.Client, baseURL string, creds hmacsig.Credentials) *HMACClient {
	return &HMACClient{
		http:     httpClient,
		baseURL:  baseURL,
		basePath: basePathOf(baseURL),
		creds:    creds,
	}
}

// basePathOf mirrors PbxWorkerGateway.php's
// rtrim(parse_url($this->baseUrl, PHP_URL_PATH) ?? ”, '/').
func basePathOf(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return strings.TrimRight(u.Path, "/")
}

var _ ReportsClient = (*HMACClient)(nil)

// QueueNames returns the real pbx-worker shape: a flat list of queue name
// strings (not objects with id/extension/name).
func (c *HMACClient) QueueNames(ctx context.Context) ([]string, error) {
	var names []string
	if err := c.get(ctx, "queue-names", nil, &names); err != nil {
		return nil, err
	}
	return names, nil
}

// AgentNames returns the real pbx-worker shape: a flat list of agent name
// strings (not objects with id/name).
func (c *HMACClient) AgentNames(ctx context.Context) ([]string, error) {
	var names []string
	if err := c.get(ctx, "agent-names", nil, &names); err != nil {
		return nil, err
	}
	return names, nil
}

func (c *HMACClient) AnsweredCalls(ctx context.Context, p ListParams) ([]AnsweredCall, error) {
	var rows []AnsweredCall
	if err := c.get(ctx, "answered-calls", listParamsToQuery(p), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *HMACClient) UnansweredCalls(ctx context.Context, p ListParams) ([]UnansweredCall, error) {
	var rows []UnansweredCall
	if err := c.get(ctx, "unanswered-calls", listParamsToQuery(p), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *HMACClient) AgentEvents(ctx context.Context, p ListParams) ([]AgentEvent, error) {
	var rows []AgentEvent
	if err := c.get(ctx, "agent-events", listParamsToQuery(p), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *HMACClient) CallSearch(ctx context.Context, p CallSearchParams) ([]CallSearchResult, error) {
	q := listParamsToQuery(p.ListParams)
	if p.CallerID != "" {
		q.Set("caller_id", p.CallerID)
	}
	if p.UniqueID != "" {
		q.Set("unique_id", p.UniqueID)
	}
	// PbxReportGateway only sends duration_operator/duration_seconds together.
	if p.DurationOperator != "" && p.DurationSeconds != nil {
		q.Set("duration_operator", p.DurationOperator)
		q.Set("duration_seconds", strconv.Itoa(*p.DurationSeconds))
	}
	var rows []CallSearchResult
	if err := c.get(ctx, "call-search", q, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// CallDetail fetches the full event timeline for one call. uniqueID is
// sent as call_uniqueid (NOT call_id), matching
// PbxReportGateway::fetchCallDetail(). The response is a flat array of
// timeline rows, not a single summary object.
func (c *HMACClient) CallDetail(ctx context.Context, uniqueID string) ([]CallDetailRow, error) {
	q := url.Values{}
	q.Set("call_uniqueid", uniqueID)
	var rows []CallDetailRow
	if err := c.get(ctx, "call-detail", q, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// listParamsToQuery builds the query params PbxReportGateway::buildParams()
// always sends: date_from/date_to as naive "Y-m-d H:i:s" local strings
// (NOT RFC3339 — neither the real gateway nor pbx-worker carry an explicit
// UTC offset here; both sides rely on the app's configured timezone, see
// README), seconds_start/seconds_end, and queues/agents only when
// non-empty. PHP's http_build_query on a numerically-indexed array emits
// queues[0]=x&queues[1]=y (not queues[]=x&queues[]=y); we match that shape.
func listParamsToQuery(p ListParams) url.Values {
	q := url.Values{}
	q.Set("date_from", p.DateFrom.Format("2006-01-02 15:04:05"))
	q.Set("date_to", p.DateTo.Format("2006-01-02 15:04:05"))
	q.Set("seconds_start", strconv.Itoa(p.SecondsStart))
	q.Set("seconds_end", strconv.Itoa(p.SecondsEnd))
	for i, name := range p.Queues {
		q.Set(fmt.Sprintf("queues[%d]", i), name)
	}
	for i, name := range p.Agents {
		q.Set(fmt.Sprintf("agents[%d]", i), name)
	}
	return q
}

// apiEnvelope is the real pbx-worker response shape: every action returns
// {"status": "ok"|..., "data": ..., "message"?: ...}. None carry a "meta"
// block — this API has no pagination at all.
type apiEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// get performs a signed GET request against /api/reports.php?action=<action>
// and decodes the envelope's "data" field into out.
func (c *HMACClient) get(ctx context.Context, action string, extra url.Values, out any) error {
	q := url.Values{}
	for k, v := range extra {
		q[k] = v
	}
	q.Set("action", action)

	reqPath := "/api/reports.php"
	reqURI := reqPath + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+reqURI, nil)
	if err != nil {
		return fmt.Errorf("pbxworker: build request for action %s: %w", action, err)
	}

	// Signed URI includes basePath: it must match the full REQUEST_URI the
	// PHP worker sees, not just the path after baseURL.
	headers := hmacsig.Sign(c.creds, http.MethodGet, c.basePath+reqURI, nil)
	applySigningHeaders(req, headers)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("pbxworker: request for action %s failed: %w", action, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("pbxworker: read response for action %s: %w", action, err)
	}

	// PbxReportGateway.php treats any 2xx as successful (not just exactly
	// 200), and distinguishes 401/403/409/429 (auth/rate-limit) from 503
	// (worker down) from other non-2xx failures. We collapse that
	// distinction into one error here but at least match the "2xx is
	// success" check.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pbxworker: action %s returned status %d: %s", action, resp.StatusCode, string(body))
	}

	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("pbxworker: decode response envelope for action %s: %w", action, err)
	}
	if env.Status != "ok" {
		msg := env.Message
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("pbxworker: action %s returned status %q: %s", action, env.Status, msg)
	}

	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("pbxworker: decode data for action %s: %w", action, err)
	}
	return nil
}

func applySigningHeaders(req *http.Request, h hmacsig.Headers) {
	req.Header.Set(hmacsig.HeaderAPIKey, h.APIKey)
	req.Header.Set(hmacsig.HeaderTimestamp, h.Timestamp)
	req.Header.Set(hmacsig.HeaderNonce, h.Nonce)
	req.Header.Set(hmacsig.HeaderBodyHash, h.BodyHash)
	req.Header.Set(hmacsig.HeaderSignature, h.Signature)
}
