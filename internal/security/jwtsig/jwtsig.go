// Package jwtsig implements the minimum of JWT this service needs: HS256
// signing and verification, using only the standard library.
//
// It is hand-rolled rather than pulled from github.com/golang-jwt/jwt
// because this repo's go.sum is not writable (owned by root), so no new
// module can be added. HS256 is a small, fully-specified construction —
// base64url(header).base64url(payload).base64url(HMAC-SHA256(signing input))
// — and the two things that usually go wrong with a hand-rolled verifier are
// guarded explicitly below:
//
//   - **Algorithm confusion.** The header's "alg" must be exactly "HS256".
//     A token claiming "none" (or an RS256 token replayed against an HMAC
//     verifier) is rejected before any signature work happens.
//   - **Non-constant-time comparison.** Signatures are compared with
//     hmac.Equal, never with ==.
//
// The token carries IDENTITY ONLY — subject and lifetime. Authority is never
// read from it; see internal/auth. That is deliberate: a token carrying
// permissions keeps granting them after a grant is revoked, until it expires.
package jwtsig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMalformed      = errors.New("jwtsig: token is not three dot-separated segments")
	ErrBadAlgorithm   = errors.New("jwtsig: unsupported or missing algorithm (only HS256)")
	ErrBadSignature   = errors.New("jwtsig: signature verification failed")
	ErrExpired        = errors.New("jwtsig: token has expired")
	ErrMissingSubject = errors.New("jwtsig: token carries no user_id")
)

// Claims is the whole payload. Identity and lifetime, nothing else.
type Claims struct {
	UserID string `json:"user_id"`
	IssuedAt int64 `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// Sign issues a token for userID, valid for ttl from now.
func Sign(secret, userID string, ttl time.Duration) (string, error) {
	if userID == "" {
		return "", ErrMissingSubject
	}
	now := time.Now().UTC()
	claims := Claims{
		UserID:    userID,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	h, err := json.Marshal(header{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", fmt.Errorf("jwtsig: encode header: %w", err)
	}
	p, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwtsig: encode claims: %w", err)
	}

	signingInput := b64(h) + "." + b64(p)
	return signingInput + "." + b64(mac(secret, signingInput)), nil
}

// Verify checks a token's algorithm, signature and expiry, and returns its
// claims. Every failure is terminal — there is no partial success and no
// permissive path.
func Verify(secret, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformed
	}

	rawHeader, err := unb64(parts[0])
	if err != nil {
		return nil, ErrMalformed
	}
	var h header
	if err := json.Unmarshal(rawHeader, &h); err != nil {
		return nil, ErrMalformed
	}
	// Checked BEFORE the signature: refusing "none" (and any asymmetric alg
	// replayed here) must not depend on signature work succeeding first.
	if h.Alg != "HS256" {
		return nil, ErrBadAlgorithm
	}

	signingInput := parts[0] + "." + parts[1]
	want := mac(secret, signingInput)
	got, err := unb64(parts[2])
	if err != nil {
		return nil, ErrBadSignature
	}
	if !hmac.Equal(got, want) {
		return nil, ErrBadSignature
	}

	rawClaims, err := unb64(parts[1])
	if err != nil {
		return nil, ErrMalformed
	}
	var c Claims
	if err := json.Unmarshal(rawClaims, &c); err != nil {
		return nil, ErrMalformed
	}
	if c.UserID == "" {
		return nil, ErrMissingSubject
	}
	if c.ExpiresAt == 0 || time.Now().UTC().Unix() >= c.ExpiresAt {
		return nil, ErrExpired
	}

	return &c, nil
}

func mac(secret, signingInput string) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(signingInput))
	return m.Sum(nil)
}

// JWT uses base64url WITHOUT padding (RFC 7515 §2).
func b64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func unb64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
