package softphone

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// A SIP password must be recoverable — digest auth needs the plaintext, so
// it cannot be one-way hashed the way a login password is (see
// internal/auth's own doc comment for the login-password case, which is
// the opposite situation). Laravel stores it via Crypt::encryptString
// (AES-256-CBC-HMAC under its own APP_KEY envelope); this package stores
// it via AES-256-GCM under its own key, which is authenticated (GCM
// detects tampering, CBC alone does not) and entirely stdlib (go.sum is
// not writable in this repo — see internal/security/jwtsig's doc comment
// for the same constraint elsewhere).
//
// The two encryption schemes are NOT compatible with each other and do not
// need to be: this is a fresh product database, never heal-crm's, so
// nothing here ever decrypts a value Laravel encrypted or vice versa.

var ErrEncryptionKeyMissing = errors.New("softphone: SIP_ENCRYPTION_KEY is not set")

// Encryptor encrypts/decrypts SIP passwords at rest.
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor builds an Encryptor from a base64-encoded 32-byte key
// (AES-256). Generate one with: openssl rand -base64 32
func NewEncryptor(base64Key string) (*Encryptor, error) {
	if base64Key == "" {
		return nil, ErrEncryptionKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("softphone: SIP_ENCRYPTION_KEY is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("softphone: SIP_ENCRYPTION_KEY must decode to 32 bytes (AES-256), got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("softphone: build cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("softphone: build GCM: %w", err)
	}
	return &Encryptor{gcm: gcm}, nil
}

// Encrypt returns base64(nonce || ciphertext || tag).
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("softphone: read nonce: %w", err)
	}
	sealed := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Returns an error on a tampered or
// wrong-key ciphertext — GCM authenticates, it does not fail open.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("softphone: ciphertext is not valid base64: %w", err)
	}
	nonceSize := e.gcm.NonceSize()
	if len(sealed) < nonceSize {
		return "", errors.New("softphone: ciphertext too short")
	}
	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("softphone: decrypt failed (wrong key or tampered data): %w", err)
	}
	return string(plaintext), nil
}
