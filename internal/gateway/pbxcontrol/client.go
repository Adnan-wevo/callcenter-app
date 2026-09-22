package pbxcontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"callcenter-service/internal/security/hmacsig"
)

// DefaultPollTimeout and DefaultPollInterval match PbxCommandService.php's
// own config defaults (config('softphone.pbx_worker.command_poll_timeout_seconds',
// 25) / command_poll_interval_ms, 500) — heal-crm's .env does not override
// either, so these ARE the real production values, not a guess.
const (
	DefaultPollTimeout  = 25 * time.Second
	DefaultPollInterval = 500 * time.Millisecond
)

// Client talks to pbx-worker's live snapshot (/api/index.php) and command
// (/api/commands.php) endpoints. Confirmed against the real staging worker
// for reads; the command submit/poll shapes are taken verbatim from
// wevetel-pbx-worker/api/commands.php (server source, not just the Laravel
// client) but have not yet been fired against a live worker — see
// docs/extraction-plan.md for the pending live-fire verification.
type Client struct {
	http     *http.Client
	baseURL  string
	basePath string
	creds    hmacsig.Credentials

	PollTimeout  time.Duration
	PollInterval time.Duration
}

func NewClient(httpClient *http.Client, baseURL string, creds hmacsig.Credentials) *Client {
	u, _ := url.Parse(baseURL)
	basePath := ""
	if u != nil {
		basePath = strings.TrimRight(u.Path, "/")
	}
	return &Client{
		http:         httpClient,
		baseURL:      baseURL,
		basePath:     basePath,
		creds:        creds,
		PollTimeout:  DefaultPollTimeout,
		PollInterval: DefaultPollInterval,
	}
}

// snapshotEnvelope is file=X's response shape
// (wevetel-pbx-worker/api/index.php's own doc comment):
//
//	{"status":"ok","data":{...},"generated_at":N,"schema_version":1}
//	{"status":"stale","data":{...},"generated_at":N,"age_seconds":N,"schema_version":1}
//	{"status":"unavailable","message":"..."}
//	{"status":"error","error":"...","message":"..."}
//
// "stale" is NOT an error — the worker still returns usable data plus how
// old it is, matching PbxWorkerGateway::statusFromResponse's PbxWorkerStatus
// enum having its own STALE case rather than treating it as a failure.
type snapshotEnvelope struct {
	Status        string          `json:"status"`
	Data          json.RawMessage `json:"data"`
	GeneratedAt   int64           `json:"generated_at"`
	AgeSeconds    int             `json:"age_seconds"`
	SchemaVersion int             `json:"schema_version"`
	Message       string          `json:"message"`
	Error         string          `json:"error"`
}

// Snapshot is a decoded file=X response, keeping the staleness metadata
// alongside the payload rather than discarding it — a caller rendering a
// live queue board needs to know an "available" count is 40 seconds old,
// not just that it is present.
type Snapshot struct {
	Stale       bool
	AgeSeconds  int
	GeneratedAt time.Time
}

func (c *Client) Queues(ctx context.Context) (QueueSnapshot, Snapshot, error) {
	var out QueueSnapshot
	snap, err := c.readFile(ctx, "queues", &out)
	return out, snap, err
}

func (c *Client) Agents(ctx context.Context) (AgentSnapshot, Snapshot, error) {
	var out AgentSnapshot
	snap, err := c.readFile(ctx, "agents", &out)
	return out, snap, err
}

func (c *Client) Calls(ctx context.Context) (CallsSnapshot, Snapshot, error) {
	var out CallsSnapshot
	snap, err := c.readFile(ctx, "calls", &out)
	return out, snap, err
}

func (c *Client) Health(ctx context.Context) (HealthSnapshot, Snapshot, error) {
	var out HealthSnapshot
	snap, err := c.readFile(ctx, "health", &out)
	return out, snap, err
}

func (c *Client) readFile(ctx context.Context, file string, out any) (Snapshot, error) {
	reqPath := "/api/index.php"
	reqURI := reqPath + "?file=" + url.QueryEscape(file)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+reqURI, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("pbxcontrol: build request for file %s: %w", file, err)
	}
	headers := hmacsig.Sign(c.creds, http.MethodGet, c.basePath+reqURI, nil)
	applyHeaders(req, headers)

	resp, err := c.http.Do(req)
	if err != nil {
		return Snapshot{}, fmt.Errorf("pbxcontrol: request for file %s failed: %w", file, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Snapshot{}, fmt.Errorf("pbxcontrol: read response for file %s: %w", file, err)
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return Snapshot{}, fmt.Errorf("pbxcontrol: worker unavailable for file %s", file)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Snapshot{}, fmt.Errorf("pbxcontrol: file %s returned status %d: %s", file, resp.StatusCode, string(body))
	}

	var env snapshotEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Snapshot{}, fmt.Errorf("pbxcontrol: decode envelope for file %s: %w", file, err)
	}
	if env.Status != "ok" && env.Status != "stale" {
		msg := env.Message
		if msg == "" {
			msg = env.Error
		}
		return Snapshot{}, fmt.Errorf("pbxcontrol: file %s returned status %q: %s", file, env.Status, msg)
	}
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return Snapshot{}, fmt.Errorf("pbxcontrol: decode data for file %s: %w", file, err)
		}
	}

	return Snapshot{
		Stale:       env.Status == "stale",
		AgeSeconds:  env.AgeSeconds,
		GeneratedAt: time.Unix(env.GeneratedAt, 0).UTC(),
	}, nil
}

// submitEnvelope is commands.php's 202 response to a POST
// (wevetel-pbx-worker/api/commands.php submitCommand()):
//
//	{"status":"queued","command_id":"cmd_...","action":"...","poll_url":"?id=...","expires_at":N}
type submitEnvelope struct {
	Status    string `json:"status"`
	CommandID string `json:"command_id"`
	Action    string `json:"action"`
	ExpiresAt int64  `json:"expires_at"`
	// Present on a 4xx rejection instead of the fields above.
	Error   string   `json:"error"`
	Message string   `json:"message"`
	Allowed []string `json:"allowed"`
}

// Submit posts a command and returns its command_id. action must be one of
// CommandProcessor.php's allowedActions; payload is one of the *Payload
// structs in types.go (or any value that marshals to the shape the worker
// expects for that action).
func (c *Client) Submit(ctx context.Context, action string, payload any) (commandID string, err error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("pbxcontrol: encode payload for action %s: %w", action, err)
	}

	reqPath := "/api/commands.php"
	reqURI := reqPath + "?action=" + url.QueryEscape(action)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+reqURI, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("pbxcontrol: build request for action %s: %w", action, err)
	}
	req.Header.Set("Content-Type", "application/json")

	headers := hmacsig.Sign(c.creds, http.MethodPost, c.basePath+reqURI, body)
	applyHeaders(req, headers)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("pbxcontrol: submit request for action %s failed: %w", action, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("pbxcontrol: read submit response for action %s: %w", action, err)
	}

	// The worker answers 202 for a successfully queued command; treat any
	// other status (400 invalid_action/invalid_request, 413 too large, 415
	// wrong content-type, 429 queue full, 503 unconfigured) as a submit
	// failure rather than trying to poll a command_id that was never issued.
	if resp.StatusCode != http.StatusAccepted {
		var env submitEnvelope
		_ = json.Unmarshal(respBody, &env)
		msg := env.Message
		if msg == "" {
			msg = string(respBody)
		}
		return "", fmt.Errorf("pbxcontrol: action %s rejected (HTTP %d): %s", action, resp.StatusCode, msg)
	}

	var env submitEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return "", fmt.Errorf("pbxcontrol: decode submit response for action %s: %w", action, err)
	}
	if env.CommandID == "" {
		return "", fmt.Errorf("pbxcontrol: action %s submitted but no command_id in response", action)
	}
	return env.CommandID, nil
}

// Poll checks one command's result once. The returned CommandResult's
// Status is "queued" or "processing" while still in flight (both are HTTP
// 202 on the wire — see serveResult() in commands.php), "success" or
// "failed" once terminal, or "not_found" if the id is unknown or expired.
func (c *Client) Poll(ctx context.Context, commandID string) (CommandResult, error) {
	reqPath := "/api/commands.php"
	reqURI := reqPath + "?id=" + url.QueryEscape(commandID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+reqURI, nil)
	if err != nil {
		return CommandResult{}, fmt.Errorf("pbxcontrol: build poll request for %s: %w", commandID, err)
	}
	headers := hmacsig.Sign(c.creds, http.MethodGet, c.basePath+reqURI, nil)
	applyHeaders(req, headers)

	resp, err := c.http.Do(req)
	if err != nil {
		return CommandResult{}, fmt.Errorf("pbxcontrol: poll request for %s failed: %w", commandID, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CommandResult{}, fmt.Errorf("pbxcontrol: read poll response for %s: %w", commandID, err)
	}

	var result CommandResult
	if err := json.Unmarshal(body, &result); err != nil {
		return CommandResult{}, fmt.Errorf("pbxcontrol: decode poll response for %s: %w", commandID, err)
	}
	if result.CommandID == "" {
		result.CommandID = commandID
	}
	return result, nil
}

// Dispatch submits a command and polls until it reaches a terminal state
// or c.PollTimeout elapses — the same submit-then-poll orchestration as
// Modules/SoftPhone/app/Services/Pbx/PbxCommandService::dispatch(). A
// timeout returns a CommandResult with Status "timeout": the worker may
// still complete the command after this call returns, exactly as the PHP
// original documents (poll() there returns 'timeout', not an error, for
// the same reason).
func (c *Client) Dispatch(ctx context.Context, action string, payload any) (CommandResult, error) {
	commandID, err := c.Submit(ctx, action, payload)
	if err != nil {
		return CommandResult{}, err
	}

	deadline := time.Now().Add(c.PollTimeout)
	ticker := time.NewTicker(c.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return CommandResult{CommandID: commandID, Status: "failed", Error: ctx.Err().Error()}, nil
		case <-ticker.C:
			result, err := c.Poll(ctx, commandID)
			if err != nil {
				return CommandResult{CommandID: commandID, Status: "failed", Error: err.Error()}, nil
			}
			if result.IsTerminal() {
				return result, nil
			}
			if result.Status == "not_found" {
				return result, nil
			}
			if time.Now().After(deadline) {
				return CommandResult{
					CommandID: commandID,
					Status:    "timeout",
					Message:   fmt.Sprintf("command did not complete within %s", c.PollTimeout),
				}, nil
			}
			// queued / processing — keep polling.
		}
	}
}

func applyHeaders(req *http.Request, h hmacsig.Headers) {
	req.Header.Set(hmacsig.HeaderAPIKey, h.APIKey)
	req.Header.Set(hmacsig.HeaderTimestamp, h.Timestamp)
	req.Header.Set(hmacsig.HeaderNonce, h.Nonce)
	req.Header.Set(hmacsig.HeaderBodyHash, h.BodyHash)
	req.Header.Set(hmacsig.HeaderSignature, h.Signature)
}
