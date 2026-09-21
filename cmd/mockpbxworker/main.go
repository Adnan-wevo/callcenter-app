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

// fixtures returns static stub data per action, shaped to match the real
// pbx-worker envelope confirmed against PbxReportGateway.php:
// {"status": "ok", "data": ...} with NO "meta"/pagination on any action.
// Kept as plain gin.H (not the real structs) to keep this mock independent
// of internal wire-format assumptions changing later.
var fixtures = map[string]gin.H{
	"queue-names": {
		"status": "ok",
		"data":   []string{"Sales", "Support"},
	},
	"agent-names": {
		"status": "ok",
		"data":   []string{"Ahmad", "Siti"},
	},
	"answered-calls": {
		"status": "ok",
		"data": []gin.H{
			{
				"datetime": "2026-09-20 09:00:12", "queue_name": "Sales", "agent_name": "Ahmad",
				"event": "COMPLETEAGENT", "uniqueid": "1758358812.101", "caller_id": "+60123456789",
				"url": "", "did": "", "ring_time": 12, "recording_file": "1758358812.101.wav",
				"hold_time": 12, "duration": 258, "position": 1, "transfer_exten": "",
				"year_month": "2026-09", "year_week": "2026-38", "date": "2026-09-20",
				"hour": 9, "day_of_week": 7, "seconds_of_day": 32412,
			},
		},
	},
	"unanswered-calls": {
		"status": "ok",
		"data": []gin.H{
			{
				"datetime": "2026-09-20 10:01:30", "queue_name": "Support", "agent_name": "",
				"event": "ABANDON", "uniqueid": "1758362490.102", "caller_id": "+60129876543",
				"url": "", "did": "", "ring_time": 90, "hold_time": 90,
				"year_month": "2026-09", "year_week": "2026-38", "date": "2026-09-20",
				"hour": 10, "day_of_week": 7,
			},
		},
	},
	"agent-events": {
		"status": "ok",
		"data": []gin.H{
			{
				"datetime": "2026-09-20 08:55:00", "queue_name": "Sales", "agent_name": "Ahmad",
				"event": "AGENTLOGIN", "info1": "", "info2": "", "info3": "",
				"timestamp": "1758358500", "uniqueid": "",
			},
		},
	},
	"call-search": {
		"status": "ok",
		"data": []gin.H{
			{
				"uniqueid": "1758358812.101", "caller_id": "+60123456789",
				"date_start": "2026-09-20 09:00:00", "date_end": "2026-09-20 09:04:30",
				"event": "COMPLETEAGENT", "agent_name": "Ahmad", "queue_name": "Sales",
				"talk_time": 258, "total_duration": 270, "wait_time": 12,
				"queue_hops": 1, "recording_file": "1758358812.101.wav",
			},
		},
	},
	"call-detail": {
		"status": "ok",
		"data": []gin.H{
			{
				"datetime": "2026-09-20 09:00:00", "queue_name": "Sales", "agent_name": "",
				"event": "ENTERQUEUE", "info1": "", "info2": "", "info3": "",
				"uniqueid": "1758358812.101", "recording_file": "",
			},
			{
				"datetime": "2026-09-20 09:00:12", "queue_name": "Sales", "agent_name": "Ahmad",
				"event": "COMPLETEAGENT", "info1": "", "info2": "", "info3": "",
				"uniqueid": "1758358812.101", "recording_file": "1758358812.101.wav",
			},
		},
	},
}
