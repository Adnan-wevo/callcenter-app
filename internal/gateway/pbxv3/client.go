// Package pbxv3 is the client for pbx-worker v3 (the Go rewrite), which
// serves a JWT-authenticated REST API rather than v2's HMAC-signed
// /api/reports.php.
//
// # Why this exists alongside gateway/pbxcontrol
//
// v2 reaches the PBX through a submit-then-poll command queue: every AMI
// action is written as a row, then polled for a result every 500ms up to a
// 25s timeout (see pbxcontrol.Client.Dispatch). That poll interval is a
// hard floor on how fast a queue pause can possibly appear, and it is the
// reason pausing felt unresponsive enough to need an optimistic UI patch.
// v3 talks to AMI directly and answers on the request, so the same action
// is one round trip with no floor under it.
//
// # Auth
//
// POST /api/v1/open/auth/login {email, password} returns {token,
// refresh_token} — note "token", not "access_token". The token's lifetime
// is chosen from the User-Agent or X-Client-Platform header: a browser is
// given JWT_TTL minutes (60 by default) while a desktop client is given a
// year. This is a backend service with nowhere to prompt for a re-login, so
// it identifies as a desktop platform and holds the long token, refreshing
// on 401 rather than on a timer.
package pbxv3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client is safe for concurrent use: the token is guarded by its own mutex
// and every request path goes through authedDo.
type Client struct {
	http     *http.Client
	baseURL  string // e.g. http://sbc.wevetel.com:8101/wmpbxworker
	email    string
	password string

	mu    sync.RWMutex
	token string
}

func New(httpClient *http.Client, baseURL, email, password string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		http:     httpClient,
		baseURL:  strings.TrimRight(baseURL, "/"),
		email:    email,
		password: password,
	}
}

// envelope is v3's uniform response wrapper: {"status": "...", "data": ...}
// with "message" carrying the reason on an error.
type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type loginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// login exchanges the configured credentials for a fresh token.
func (c *Client) login(ctx context.Context) (string, error) {
	body, err := json.Marshal(map[string]string{"email": c.email, "password": c.password})
	if err != nil {
		return "", fmt.Errorf("pbxv3: encode login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/open/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("pbxv3: build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Asks for the long-lived token — see the package comment.
	req.Header.Set("X-Client-Platform", "linux")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("pbxv3: login: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pbxv3: login rejected (%d): %s", resp.StatusCode, summarize(raw))
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", fmt.Errorf("pbxv3: decode login envelope: %w", err)
	}
	var out loginResponse
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return "", fmt.Errorf("pbxv3: decode login data: %w", err)
	}
	if out.Token == "" {
		return "", fmt.Errorf("pbxv3: login returned no token")
	}
	return out.Token, nil
}

// currentToken returns a usable token, logging in if none is cached yet.
func (c *Client) currentToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	tok := c.token
	c.mu.RUnlock()
	if tok != "" {
		return tok, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" { // another goroutine won the race
		return c.token, nil
	}
	tok, err := c.login(ctx)
	if err != nil {
		return "", err
	}
	c.token = tok
	return tok, nil
}

// forget drops a token that the server has rejected, so the next call logs
// in again. Takes the token it saw rejected so a concurrent caller that has
// already replaced it does not have its fresh one discarded.
func (c *Client) forget(rejected string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == rejected {
		c.token = ""
	}
}

// do performs a request against path, retrying once on 401 with a fresh
// token — which is how an expired token is handled, since there is no
// timer telling this client when that happens.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var encoded []byte
	if body != nil {
		var err error
		if encoded, err = json.Marshal(body); err != nil {
			return fmt.Errorf("pbxv3: encode %s body: %w", path, err)
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.currentToken(ctx)
		if err != nil {
			return err
		}

		status, raw, err := c.send(ctx, method, path, query, encoded, token)
		if err != nil {
			return err
		}

		if status == http.StatusUnauthorized && attempt == 0 {
			c.forget(token)
			continue
		}
		if status < 200 || status > 299 {
			return fmt.Errorf("pbxv3: %s %s: %d %s", method, path, status, summarize(raw))
		}
		if out == nil {
			return nil
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("pbxv3: decode %s envelope: %w", path, err)
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("pbxv3: decode %s data: %w", path, err)
		}
		return nil
	}
	return fmt.Errorf("pbxv3: %s %s: still unauthorized after re-login", method, path)
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values, body []byte, token string) (int, []byte, error) {
	u := c.baseURL + "/api/v1" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("pbxv3: build %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("pbxv3: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("pbxv3: read %s %s: %w", method, path, err)
	}
	return resp.StatusCode, raw, nil
}

// summarize pulls the human-readable reason out of an error envelope, so a
// wrapped error says "extension not found" rather than carrying a whole
// JSON document into a log line.
func summarize(raw []byte) string {
	var env envelope
	if err := json.Unmarshal(raw, &env); err == nil && env.Message != "" {
		return env.Message
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
