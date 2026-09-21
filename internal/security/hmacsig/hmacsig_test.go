package hmacsig

import "testing"

// TestSignMatchesPHP is a known-answer test against an independently
// computed value (Python: base64.b64encode(hmac.new(secret.encode(),
// canonical.encode(), hashlib.sha256).digest())), matching
// Modules/SoftPhone/app/Support/HmacSigner.php's
// base64_encode(hash_hmac('sha256', $canonical, $secret, true)). This
// guards against the signature encoding silently regressing back to hex,
// which broke 100% of requests in both directions until fixed.
func TestSignMatchesPHP(t *testing.T) {
	canonical := "GET\n/api/reports.php?action=queue-names\n1758358812\nabc123\ndeadbeef"
	secret := "local-dev-secret"
	const wantBase64 = "0lXyJcWp6ulDLn0nHALxXbjfgyr28vnPBu0Fn0IR1sU="

	got := sign(secret, canonical)
	if got != wantBase64 {
		t.Fatalf("sign() = %q, want %q (base64, matching PHP's base64_encode(hash_hmac(..., true)) — NOT hex)", got, wantBase64)
	}
}

func TestBodyHashIsHex(t *testing.T) {
	// PHP's hash('sha256', $body) (no raw-output flag) is lowercase hex —
	// unlike the signature above, this one must stay hex, not base64.
	got := BodyHash([]byte(""))
	const wantHex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != wantHex {
		t.Fatalf("BodyHash(\"\") = %q, want %q", got, wantHex)
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	creds := Credentials{APIKey: "k", Secret: "s"}
	h := Sign(creds, "POST", "/api/v1/callback/attempts", []byte(`{"phone_number":"+60123456789"}`))
	if err := Verify(creds, "POST", "/api/v1/callback/attempts", []byte(`{"phone_number":"+60123456789"}`), h, 300*1e9); err != nil {
		t.Fatalf("Verify() failed on a request just signed by Sign(): %v", err)
	}
}
