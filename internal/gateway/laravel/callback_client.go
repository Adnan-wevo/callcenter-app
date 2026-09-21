// Package laravel is the one outbound write-path this service has: recording
// an ad-hoc callback attempt back in the Laravel monolith when an agent
// clicks "callback" from the unanswered-calls report.
//
// Before extraction, CallCenter called the Callback module's function
// in-process. After extraction, that's no longer possible (different
// process/service), so it becomes an HTTP call back to Laravel instead.
//
// TODO: everything about this endpoint is unconfirmed — path, payload
// shape, response shape, and even whether Laravel will require this exact
// HMAC scheme or something else on its inbound side. Confirm all of it
// once the reference code lands; this is a best-effort placeholder so the
// rest of the service has something to call.
package laravel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"callcenter-service/internal/security/hmacsig"
)

// CallbackRequest is an ASSUMPTION of what Laravel's Callback module needs
// to record an ad-hoc callback attempt. Confirm field names/types against
// the original in-process call once reference code lands.
type CallbackRequest struct {
	UnansweredCallID string  `json:"unanswered_call_id"`
	QueueID           string  `json:"queue_id"`
	AgentID           string  `json:"agent_id"`
	AttemptedAt        time.Time `json:"attempted_at"`
	Note                *string  `json:"note,omitempty"`
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
