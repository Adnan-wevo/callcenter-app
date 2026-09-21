// Package middleware holds Gin middleware for this service.
package middleware

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/security/hmacsig"
)

// nonceCache is a minimal in-memory replay guard: once a nonce is seen it is
// rejected until it ages out past the tolerance window.
//
// TODO: this is single-instance only. If this service is ever run with more
// than one replica (or restarted mid-window), replay protection resets /
// stops being shared. Move to Redis (or similar) before that happens — not
// needed for the current single-instance local-testing scope.
type nonceCache struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func newNonceCache() *nonceCache {
	return &nonceCache{seen: make(map[string]time.Time)}
}

func (c *nonceCache) seenBefore(nonce string, tolerance time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for n, t := range c.seen {
		if now.Sub(t) > tolerance {
			delete(c.seen, n)
		}
	}

	if _, ok := c.seen[nonce]; ok {
		return true
	}
	c.seen[nonce] = now
	return false
}

// HMACVerify checks inbound requests against the shared HMAC scheme
// documented in internal/security/hmacsig. See config.Config.IncomingAuthDisabled
// to bypass this during local development.
func HMACVerify(creds hmacsig.Credentials, tolerance time.Duration, disabled bool) gin.HandlerFunc {
	cache := newNonceCache()

	return func(c *gin.Context) {
		if disabled {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			apires.Error(c, http.StatusBadRequest, "unable to read request body", nil)
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		h := hmacsig.Headers{
			APIKey:    c.GetHeader(hmacsig.HeaderAPIKey),
			Timestamp: c.GetHeader(hmacsig.HeaderTimestamp),
			Nonce:     c.GetHeader(hmacsig.HeaderNonce),
			BodyHash:  c.GetHeader(hmacsig.HeaderBodyHash),
			Signature: c.GetHeader(hmacsig.HeaderSignature),
		}

		// ASSUMPTION: URI = request URI (path + query), no scheme/host.
		// Confirm against the real Laravel/pbx-worker implementation.
		uri := c.Request.URL.RequestURI()

		if err := hmacsig.Verify(creds, c.Request.Method, uri, body, h, tolerance); err != nil {
			apires.Error(c, http.StatusUnauthorized, "request signature invalid", nil)
			c.Abort()
			return
		}

		if h.Nonce == "" || cache.seenBefore(h.Nonce, tolerance) {
			apires.Error(c, http.StatusUnauthorized, "nonce already used or missing", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
