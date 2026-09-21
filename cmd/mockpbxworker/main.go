// mockpbxworker stands in for the real pbx-worker v2 service during local
// testing (see docker-compose.yml). It serves canned fixture data at
// /api/reports.php?action=... and enforces the same HMAC scheme the real
// worker uses, so the full signing round-trip can be exercised end-to-end
// without touching the real infra.
package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/security/hmacsig"
)

func main() {
	port := getenv("PORT", "8081")
	creds := hmacsig.Credentials{
		APIKey: getenv("HMAC_API_KEY", "local-dev-key"),
		Secret: getenv("HMAC_SECRET", "local-dev-secret"),
	}
	tolerance := 5 * time.Minute

	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	r.GET("/api/reports.php", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)

		h := hmacsig.Headers{
			APIKey:    c.GetHeader(hmacsig.HeaderAPIKey),
			Timestamp: c.GetHeader(hmacsig.HeaderTimestamp),
			Nonce:     c.GetHeader(hmacsig.HeaderNonce),
			BodyHash:  c.GetHeader(hmacsig.HeaderBodyHash),
			Signature: c.GetHeader(hmacsig.HeaderSignature),
		}
		uri := c.Request.URL.RequestURI()
		if err := hmacsig.Verify(creds, c.Request.Method, uri, body, h, tolerance); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid signature", "error": err.Error()})
			return
		}

		action := c.Query("action")
		fixture, ok := fixtures[action]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"message": "unknown action: " + action})
			return
		}
		c.JSON(http.StatusOK, fixture)
	})

	log.Printf("mockpbxworker listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// fixtures returns static stub data per action, shaped to match
// internal/gateway/pbxworker's expected {"data": ...} / {"data": ..., "meta": ...}
// envelope. Kept as plain gin.H (not the real structs) to keep this mock
// independent of internal wire-format assumptions changing later.
var fixtures = map[string]gin.H{
	"queue-names": {
		"data": []gin.H{
			{"id": "1", "extension": "6001", "name": "Sales"},
			{"id": "2", "extension": "6002", "name": "Support"},
		},
	},
	"agent-names": {
		"data": []gin.H{
			{"id": "101", "name": "Ahmad"},
			{"id": "102", "name": "Siti"},
		},
	},
	"answered-calls": {
		"data": []gin.H{
			{
				"id": "call-1001", "queue_id": "1", "queue_name": "Sales",
				"agent_id": "101", "agent_name": "Ahmad", "caller_id": "+60123456789",
				"entered_at": "2026-09-20T09:00:00Z", "answered_at": "2026-09-20T09:00:12Z", "ended_at": "2026-09-20T09:04:30Z",
				"wait_seconds": 12, "talk_seconds": 258,
			},
		},
		"meta": gin.H{"current_page": 1, "per_page": 25, "total": 1, "last_page": 1},
	},
	"unanswered-calls": {
		"data": []gin.H{
			{
				"id": "call-1002", "queue_id": "2", "queue_name": "Support",
				"caller_id": "+60129876543",
				"entered_at": "2026-09-20T10:00:00Z", "abandoned_at": "2026-09-20T10:01:30Z",
				"wait_seconds": 90, "reason": "ABANDONED",
			},
		},
		"meta": gin.H{"current_page": 1, "per_page": 25, "total": 1, "last_page": 1},
	},
	"agent-events": {
		"data": []gin.H{
			{"id": "evt-1", "agent_id": "101", "agent_name": "Ahmad", "queue_id": "1", "event_type": "LOGIN", "event_time": "2026-09-20T08:55:00Z"},
		},
		"meta": gin.H{"current_page": 1, "per_page": 25, "total": 1, "last_page": 1},
	},
	"call-search": {
		"data": []gin.H{
			{"id": "call-1001", "queue_id": "1", "agent_id": "101", "caller_id": "+60123456789", "status": "ANSWERED", "started_at": "2026-09-20T09:00:00Z", "ended_at": "2026-09-20T09:04:30Z"},
		},
		"meta": gin.H{"current_page": 1, "per_page": 25, "total": 1, "last_page": 1},
	},
	"call-detail": {
		"data": gin.H{
			"id": "call-1001", "queue_id": "1", "agent_id": "101", "caller_id": "+60123456789",
			"status": "ANSWERED", "started_at": "2026-09-20T09:00:00Z", "ended_at": "2026-09-20T09:04:30Z",
			"wait_seconds": 12, "talk_seconds": 258, "recording_url": "http://mock-pbx-worker:8081/recordings/call-1001.wav",
			"events": []gin.H{
				{"id": "evt-1", "agent_id": "101", "agent_name": "Ahmad", "queue_id": "1", "event_type": "ANSWERED", "event_time": "2026-09-20T09:00:12Z"},
			},
		},
	},
}
