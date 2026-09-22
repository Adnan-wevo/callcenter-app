package softphone

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testKey() string {
	return base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901")[:32])
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := NewEncryptor(testKey())
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	ciphertext, err := enc.Encrypt("hunter2-sip-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if strings.Contains(ciphertext, "hunter2") {
		t.Fatal("ciphertext contains the plaintext password")
	}

	got, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != "hunter2-sip-password" {
		t.Fatalf("Decrypt() = %q, want %q", got, "hunter2-sip-password")
	}
}

func TestEncryptIsNondeterministic(t *testing.T) {
	enc, _ := NewEncryptor(testKey())
	a, _ := enc.Encrypt("same-password")
	b, _ := enc.Encrypt("same-password")
	if a == b {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext — nonce is not random")
	}
}

func TestDecryptRejectsTampering(t *testing.T) {
	enc, _ := NewEncryptor(testKey())
	ciphertext, _ := enc.Encrypt("original")

	raw, _ := base64.StdEncoding.DecodeString(ciphertext)
	raw[len(raw)-1] ^= 0xFF // flip the last byte of the auth tag
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, err := enc.Decrypt(tampered); err == nil {
		t.Fatal("Decrypt accepted tampered ciphertext")
	}
}

func TestNewEncryptorRejectsBadKeys(t *testing.T) {
	if _, err := NewEncryptor(""); err != ErrEncryptionKeyMissing {
		t.Errorf("empty key: err = %v, want ErrEncryptionKeyMissing", err)
	}
	if _, err := NewEncryptor("not-base64!!!"); err == nil {
		t.Error("invalid base64 key: expected an error")
	}
	if _, err := NewEncryptor(base64.StdEncoding.EncodeToString([]byte("too-short"))); err == nil {
		t.Error("16-byte key (not 32): expected an error")
	}
}
