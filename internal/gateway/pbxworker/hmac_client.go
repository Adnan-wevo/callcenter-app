package pbxworker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"callcenter-service/internal/security/hmacsig"
)

// HMACClient talks to pbx-worker v2 (PHP), base path /api/reports.php,
// action=<name> query param, HMAC-SHA256 signed per hmacsig.
//
// ASSUMPTION: all actions are GET requests with filters as query params,
// and all return {"data": ..., "meta": {...}} for lists — matching the
// Laravel API Resource convention this service itself follows. Confirm
// against the real pbx-worker responses once reference code lands.
type HMACClient struct {
	http    *http.Client
	baseURL string
	creds   hmacsig.Credentials
}

func NewHMACClient(httpClient *http.Client, baseURL string, creds hmacsig.Credentials) *HMACClient {
	return &HMACClient{http: httpClient, baseURL: baseURL, creds: creds}
}

var _ ReportsClient = (*HMACClient)(nil)

func (c *HMACClient) QueueNames(ctx context.Context) ([]QueueName, error) {
	var out struct {
		Data []QueueName `json:"data"`
	}
	if err := c.get(ctx, "queue-names", nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (c *HMACClient) AgentNames(ctx context.Context) ([]AgentName, error) {
	var out struct {
		Data []AgentName `json:"data"`
	}
	if err := c.get(ctx, "agent-names", nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (c *HMACClient) AnsweredCalls(ctx context.Context, p ListParams) ([]AnsweredCall, Pagination, error) {
	var out struct {
		Data []AnsweredCall `json:"data"`
		Meta Pagination     `json:"meta"`
	}
	if err := c.get(ctx, "answered-calls", listParamsToQuery(p), &out); err != nil {
		return nil, Pagination{}, err
	}
	return out.Data, out.Meta, nil
}

func (c *HMACClient) UnansweredCalls(ctx context.Context, p ListParams) ([]UnansweredCall, Pagination, error) {
	var out struct {
		Data []UnansweredCall `json:"data"`
		Meta Pagination       `json:"meta"`
	}
	if err := c.get(ctx, "unanswered-calls", listParamsToQuery(p), &out); err != nil {
		return nil, Pagination{}, err
	}
	return out.Data, out.Meta, nil
}

func (c *HMACClient) AgentEvents(ctx context.Context, p ListParams) ([]AgentEvent, Pagination, error) {
	var out struct {
		Data []AgentEvent `json:"data"`
		Meta Pagination   `json:"meta"`
	}
	if err := c.get(ctx, "agent-events", listParamsToQuery(p), &out); err != nil {
		return nil, Pagination{}, err
	}
	return out.Data, out.Meta, nil
}

func (c *HMACClient) CallSearch(ctx context.Context, p CallSearchParams) ([]CallSummary, Pagination, error) {
	q := listParamsToQuery(p.ListParams)
	if p.CallerID != "" {
		q.Set("caller_id", p.CallerID)
	}
	if p.Status != "" {
		q.Set("status", p.Status)
	}
	var out struct {
		Data []CallSummary `json:"data"`
		Meta Pagination    `json:"meta"`
	}
	if err := c.get(ctx, "call-search", q, &out); err != nil {
		return nil, Pagination{}, err
	}
	return out.Data, out.Meta, nil
}

func (c *HMACClient) CallDetail(ctx context.Context, callID string) (*CallDetail, error) {
	q := url.Values{}
	q.Set("call_id", callID) // ASSUMPTION: param name for the call-detail action
	var out struct {
		Data CallDetail `json:"data"`
	}
	if err := c.get(ctx, "call-detail", q, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

func listParamsToQuery(p ListParams) url.Values {
	q := url.Values{}
	if p.QueueID != "" {
		q.Set("queue_id", p.QueueID)
	}
	if p.AgentID != "" {
		q.Set("agent_id", p.AgentID)
	}
	if p.From != nil {
		q.Set("from", p.From.Format(time.RFC3339))
	}
	if p.To != nil {
		q.Set("to", p.To.Format(time.RFC3339))
	}
	if p.Page > 0 {
		q.Set("page", strconv.Itoa(p.Page))
	}
	if p.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(p.PerPage))
	}
	return q
}

// get performs a signed GET request against /api/reports.php?action=<action>&...
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

	headers := hmacsig.Sign(c.creds, http.MethodGet, reqURI, nil)
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pbxworker: action %s returned status %d: %s", action, resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("pbxworker: decode response for action %s: %w", action, err)
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
