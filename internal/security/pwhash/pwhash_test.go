package pwhash

import (
	"encoding/hex"
	"testing"
)

// Published PBKDF2-HMAC-SHA256 test vectors. If these fail, the KDF is wrong
// and every hash this package has ever written is suspect — which is exactly
// why a hand-written KDF must be pinned to vectors it did not produce itself.
func TestPBKDF2KnownVectors(t *testing.T) {
	cases := []struct {
		password   string
		salt       string
		iterations int
		keyLen     int
		want       string
	}{
		{"password", "salt", 1, 32, "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{"password", "salt", 2, 32, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{"password", "salt", 4096, 32, "c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a"},
		{"passwd", "salt", 1, 64, "55ac046e56e3089fec1691c22544b605f94185216dde0465e68b9d57c20dacbc49ca9cccf179b645991664b39d77ef317c71b845b1e30bd509112041d3a19783"},
	}

	for _, c := range cases {
		got := hex.EncodeToString(pbkdf2(c.password, []byte(c.salt), c.iterations, c.keyLen))
		if got != c.want {
			t.Errorf("pbkdf2(%q, %q, %d, %d)\n got %s\nwant %s",
				c.password, c.salt, c.iterations, c.keyLen, got, c.want)
		}
	}
}

func TestHashVerifyRoundTrip(t *testing.T) {
	encoded, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if err := Verify(encoded, "correct horse battery staple"); err != nil {
		t.Fatalf("Verify with correct password: %v", err)
	}
	if err := Verify(encoded, "wrong"); err != ErrMismatch {
		t.Fatalf("Verify with wrong password: err = %v, want ErrMismatch", err)
	}
}

// Two hashes of the same password must differ — otherwise the salt is not
// doing its job and the store is vulnerable to precomputation.
func TestHashIsSalted(t *testing.T) {
	a, _ := Hash("same")
	b, _ := Hash("same")
	if a == b {
		t.Fatal("two hashes of the same password are identical; salt is not random")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{
		"",
		"notahash",
		"bcrypt$10$abc$def",
		"pbkdf2-sha256$abc$c2FsdA$a2V5", // non-numeric iterations
		"pbkdf2-sha256$0$c2FsdA$a2V5",   // zero iterations
		"pbkdf2-sha256$1000$!!!$a2V5",   // bad salt base64
		"pbkdf2-sha256$1000$c2FsdA",     // too few fields
	} {
		if err := Verify(bad, "x"); err != ErrInvalidHash {
			t.Errorf("Verify(%q): err = %v, want ErrInvalidHash", bad, err)
		}
	}
}
