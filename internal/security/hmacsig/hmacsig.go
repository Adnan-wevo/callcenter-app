// Package hmacsig implements the HMAC-SHA256 request signing scheme shared by
// the existing Laravel <-> pbx-worker (v2) integration.
//
// CONFIRMED against Modules/SoftPhone/app/Support/HmacSigner.php and
// Modules/SoftPhone/app/Http/Middleware/VerifyPbxWebhookHmac.php:
//
//	Headers:   X-Api-Key, X-Timestamp, X-Nonce, X-Body-Hash, X-Signature
//	Canonical: METHOD\nURI\nTIMESTAMP\nNONCE\nBODY_HASH
//	BodyHash:  lowercase hex SHA-256 of the raw request body
//	Signature: base64(HMAC_SHA256(secret, canonical))  -- NOT hex
//	Timestamp: Unix epoch seconds, decimal string (PHP's (string) time())
//	Nonce:     16 random bytes, lowercase hex (32 chars)
//	Tolerance: 300s clock skew, 600s nonce-replay TTL on the PHP side
//	Key:       secret used as raw bytes directly, no KDF/decoding
//
// It is used both for outbound calls this service makes (to pbx-worker and
// back to Laravel) and for verifying inbound calls made to this service.
package hmacsig

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

var (
	ErrInvalidAPIKey       = errors.New("hmacsig: unknown api key")
	ErrInvalidTimestamp    = errors.New("hmacsig: malformed timestamp header")
	ErrTimestampOutOfRange = errors.New("hmacsig: timestamp outside allowed tolerance")
	ErrBodyHashMismatch    = errors.New("hmacsig: body hash does not match request body")
	ErrInvalidSignature    = errors.New("hmacsig: signature verification failed")
)

// Credentials identifies one caller/callee pair for a signed relationship
// (e.g. this service <-> pbx-worker, or this service <-> Laravel).
type Credentials struct {
	APIKey string
	Secret string
}

// Headers holds the five signing headers as sent/received on the wire.
type Headers struct {
	APIKey    string
	Timestamp string
	Nonce     string
	BodyHash  string
	Signature string
}

// HTTP header names, per spec.
const (
	HeaderAPIKey    = "X-Api-Key"
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
	HeaderBodyHash  = "X-Body-Hash"
	HeaderSignature = "X-Signature"
)

// BodyHash returns the lowercase hex SHA-256 digest of body.
func BodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// CanonicalString builds the string that gets HMAC-signed.
func CanonicalString(method, uri, timestamp, nonce, bodyHash string) string {
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, uri, timestamp, nonce, bodyHash)
}

// sign returns base64(HMAC-SHA256(secret, canonical)) — matching PHP's
// base64_encode(hash_hmac('sha256', $canonical, $secret, true)). This must
// stay base64, not hex: BodyHash above is hex (matches PHP's hash()
// without raw output), but the final signature is base64 on both sides.
func sign(secret, canonical string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Sign produces a fresh set of signing headers for an outbound request.
func Sign(creds Credentials, method, uri string, body []byte) Headers {
	ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	nonce := randomNonce()
	bh := BodyHash(body)
	canonical := CanonicalString(method, uri, ts, nonce, bh)
	return Headers{
		APIKey:    creds.APIKey,
		Timestamp: ts,
		Nonce:     nonce,
		BodyHash:  bh,
		Signature: sign(creds.Secret, canonical),
	}
}

// Verify checks an inbound request's signing headers against creds.
// tolerance bounds how far the timestamp may drift from "now" in either
// direction (replay/staleness window). Nonce replay checking is NOT done
// here — callers (see internal/middleware) are expected to track seen
// nonces themselves within the tolerance window.
func Verify(creds Credentials, method, uri string, body []byte, h Headers, tolerance time.Duration) error {
	if h.APIKey == "" || h.APIKey != creds.APIKey {
		return ErrInvalidAPIKey
	}

	tsUnix, err := strconv.ParseInt(h.Timestamp, 10, 64)
	if err != nil {
		return ErrInvalidTimestamp
	}
	delta := time.Since(time.Unix(tsUnix, 0))
	if delta < 0 {
		delta = -delta
	}
	if delta > tolerance {
		return ErrTimestampOutOfRange
	}

	expectedBodyHash := BodyHash(body)
	if !hmac.Equal([]byte(h.BodyHash), []byte(expectedBodyHash)) {
		return ErrBodyHashMismatch
	}

	canonical := CanonicalString(method, uri, h.Timestamp, h.Nonce, h.BodyHash)
	expectedSig := sign(creds.Secret, canonical)
	if !hmac.Equal([]byte(h.Signature), []byte(expectedSig)) {
		return ErrInvalidSignature
	}

	return nil
}

func randomNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is effectively unrecoverable; a predictable
		// nonce would be a security bug, so fail loudly instead.
		panic("hmacsig: failed to read random bytes for nonce: " + err.Error())
	}
	return hex.EncodeToString(b)
}
