package jwtsig

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	tok, err := Sign("secret", "user-1", time.Hour)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	c, err := Verify("secret", tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if c.UserID != "user-1" {
		t.Fatalf("UserID = %q, want %q", c.UserID, "user-1")
	}
}

// A token signed against an HS256 verifier must be rejected outright when its
// header claims a different algorithm — including "none", the classic bypass.
func TestVerifyRejectsAlgorithmConfusion(t *testing.T) {
	for _, alg := range []string{"none", "None", "RS256", "HS512", ""} {
		h, _ := json.Marshal(map[string]string{"alg": alg, "typ": "JWT"})
		p, _ := json.Marshal(Claims{UserID: "attacker", ExpiresAt: time.Now().Add(time.Hour).Unix()})
		signingInput := b64(h) + "." + b64(p)
		// Sign it correctly with the real secret: only the `alg` header differs,
		// so a verifier that skips the algorithm check would accept this.
		forged := signingInput + "." + b64(mac("secret", signingInput))

		if _, err := Verify("secret", forged); err != ErrBadAlgorithm {
			t.Fatalf("alg=%q: err = %v, want ErrBadAlgorithm", alg, err)
		}
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	tok, _ := Sign("secret", "user-1", time.Hour)
	if _, err := Verify("other-secret", tok); err != ErrBadSignature {
		t.Fatalf("err = %v, want ErrBadSignature", err)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	tok, _ := Sign("secret", "user-1", time.Hour)
	parts := strings.Split(tok, ".")
	evil, _ := json.Marshal(Claims{UserID: "admin", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	tampered := parts[0] + "." + b64(evil) + "." + parts[2]

	if _, err := Verify("secret", tampered); err != ErrBadSignature {
		t.Fatalf("err = %v, want ErrBadSignature", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	tok, _ := Sign("secret", "user-1", -time.Second)
	if _, err := Verify("secret", tok); err != ErrExpired {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, tok := range []string{"", "a", "a.b", "a.b.c.d", "!!.!!.!!"} {
		if _, err := Verify("secret", tok); err == nil {
			t.Fatalf("token %q: expected an error, got nil", tok)
		}
	}
}

// JWT segments are base64url with no padding (RFC 7515 §2); a '=' in the
// output would make the token invalid for other parsers.
func TestSegmentsAreUnpaddedBase64URL(t *testing.T) {
	tok, _ := Sign("secret", "user-1", time.Hour)
	if strings.Contains(tok, "=") {
		t.Fatalf("token contains padding: %q", tok)
	}
	for _, part := range strings.Split(tok, ".") {
		if _, err := base64.RawURLEncoding.DecodeString(part); err != nil {
			t.Fatalf("segment %q is not raw base64url: %v", part, err)
		}
	}
}
