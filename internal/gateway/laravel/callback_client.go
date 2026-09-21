// Package laravel is the one outbound write-path this service has: recording
// an ad-hoc callback attempt back in the Laravel monolith when an agent
// clicks "callback" from the unanswered-calls report.
//
// Before extraction, CallCenter called the Callback module's function
// in-process. After extraction, that's no longer possible (different
// process/service), so it becomes an HTTP call back to Laravel instead.
//
// The PAYLOAD is now confirmed against the real Action (see CallbackRequest).
// The ENDPOINT is not: Modules/Callback has no routes/api.php, no controller
// and no mapApiRoutes() — the Laravel side of this does not exist yet and
// must be built. The path to build it at is open decision D5; the convention
// to mirror is Modules/SoftPhone's HMAC-verified webhook route. See
// docs/extraction-plan.md §6.
package laravel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"callcenter-service/internal/security/hmacsig"
)

// CallbackRequest mirrors the real Action's signature —
// RecordAdHocCallbackAttempt::handle(string $phoneNumber, string $agentId)
// in Modules/Callback/app/Actions. Those two arguments are the whole input;
// the Action hardcodes candidate_id (null), dialed_at (now) and outcome
// (Pending) itself.
//
// An earlier version of this struct carried unanswered_call_id, queue_id,
// attempted_at and note. None of them exist in the Action's signature, and
// phone_number — the one field that matters — was missing entirely.
type CallbackRequest struct {
	PhoneNumber string `json:"phone_number"`
	AgentID     string `json:"agent_id"`
}

// CallbackClient records an ad-hoc callback attempt in Laravel.
type CallbackClient interface {
	RecordAdHocCallback(ctx context.Context, req CallbackRequest) error
}

type HMACCallbackClient struct {
	http    *http.Client
	baseURL string
	path    string
	creds   hmacsig.Credentials
}

func NewHMACCallbackClient(httpClient *http.Client, baseURL, path string, creds hmacsig.Credentials) *HMACCallbackClient {
	return &HMACCallbackClient{http: httpClient, baseURL: baseURL, path: path, creds: creds}
}

var _ CallbackClient = (*HMACCallbackClient)(nil)

func (c *HMACCallbackClient) RecordAdHocCallback(ctx context.Context, reqBody CallbackRequest) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("laravel: encode callback request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("laravel: build callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	headers := hmacsig.Sign(c.creds, http.MethodPost, c.path, body)
	req.Header.Set(hmacsig.HeaderAPIKey, headers.APIKey)
	req.Header.Set(hmacsig.HeaderTimestamp, headers.Timestamp)
	req.Header.Set(hmacsig.HeaderNonce, headers.Nonce)
	req.Header.Set(hmacsig.HeaderBodyHash, headers.BodyHash)
	req.Header.Set(hmacsig.HeaderSignature, headers.Signature)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("laravel: callback request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("laravel: callback request returned status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
