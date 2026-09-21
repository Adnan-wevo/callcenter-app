// hmacsign is a small CLI helper for manually testing the running service
// with curl. It prints the five signing headers for a given request so you
// can paste them straight into a curl command.
//
// Usage:
//
//	go run ./cmd/hmacsign -key local-dev-key -secret local-dev-secret \
//	    -method GET -uri "/api/v1/call-center/queues"
//
//	go run ./cmd/hmacsign -key local-dev-key -secret local-dev-secret \
//	    -method POST -uri "/api/v1/call-center/unanswered-calls/call-1002/callback" \
//	    -body '{"queue_id":"2","agent_id":"101"}'
package main

import (
	"flag"
	"fmt"

	"callcenter-service/internal/security/hmacsig"
)

func main() {
	key := flag.String("key", "local-dev-key", "API key")
	secret := flag.String("secret", "local-dev-secret", "shared secret")
	method := flag.String("method", "GET", "HTTP method")
	uri := flag.String("uri", "/", "request URI (path + query string)")
	body := flag.String("body", "", "request body (empty for GET)")
	flag.Parse()

	h := hmacsig.Sign(hmacsig.Credentials{APIKey: *key, Secret: *secret}, *method, *uri, []byte(*body))

	fmt.Printf("%s: %s\n", hmacsig.HeaderAPIKey, h.APIKey)
	fmt.Printf("%s: %s\n", hmacsig.HeaderTimestamp, h.Timestamp)
	fmt.Printf("%s: %s\n", hmacsig.HeaderNonce, h.Nonce)
	fmt.Printf("%s: %s\n", hmacsig.HeaderBodyHash, h.BodyHash)
	fmt.Printf("%s: %s\n", hmacsig.HeaderSignature, h.Signature)

	fmt.Println("\ncurl example:")
	curl := fmt.Sprintf(
		"curl -H '%s: %s' -H '%s: %s' -H '%s: %s' -H '%s: %s' -H '%s: %s'",
		hmacsig.HeaderAPIKey, h.APIKey,
		hmacsig.HeaderTimestamp, h.Timestamp,
		hmacsig.HeaderNonce, h.Nonce,
		hmacsig.HeaderBodyHash, h.BodyHash,
		hmacsig.HeaderSignature, h.Signature,
	)
	if *body != "" {
		curl += fmt.Sprintf(" -H 'Content-Type: application/json' -d '%s'", *body)
	}
	curl += fmt.Sprintf(" -X %s 'http://localhost:8080%s'", *method, *uri)
	fmt.Println(curl)
}
