// mocklaravel stands in for the Laravel monolith during local testing. It
// only implements the one thing this service calls back into Laravel for:
// recording an ad-hoc callback attempt (see internal/gateway/laravel).
// The path here MUST match LARAVEL_CALLBACK_PATH — it is a guess (TODO in
// internal/gateway/laravel) until the real Laravel route is confirmed.
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/security/hmacsig"
)

func main() {
	port := getenv("PORT", "8082")
	callbackPath := getenv("CALLBACK_PATH", "/api/internal/call-center/callback-attempts")
	creds := hmacsig.Credentials{
		APIKey: getenv("HMAC_API_KEY", "local-dev-key"),
		Secret: getenv("HMAC_SECRET", "local-dev-secret"),
	}
	tolerance := 5 * time.Minute

	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	r.POST(callbackPath, func(c *gin.Context) {
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

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid json body"})
			return
		}
		log.Printf("mocklaravel: received callback attempt: %+v", payload)

		c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "received"}})
	})

	log.Printf("mocklaravel listening on :%s (callback path %s)", port, callbackPath)
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
