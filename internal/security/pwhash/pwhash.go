// Package pwhash stores and verifies passwords using PBKDF2-HMAC-SHA256
// (RFC 8018), with only the standard library.
//
// bcrypt/argon2 would be the ordinary choice, but both live in
// golang.org/x/crypto and this repo's go.sum is not writable (owned by
// root), so no new module can be added. PBKDF2 is the one credible KDF
// buildable from stdlib alone: it is nothing but repeated HMAC, so there is
// no primitive being invented here, and the implementation is pinned to
// published RFC test vectors in pwhash_test.go.
//
// Encoded form is self-describing so a later migration to bcrypt/argon2 can
// recognise and upgrade old hashes in place:
//
//	pbkdf2-sha256$<iterations>$<base64 salt>$<base64 key>
package pwhash

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// DefaultIterations is deliberately high: PBKDF2-HMAC-SHA256 is cheaper per
// guess than bcrypt at equivalent settings, so the iteration count carries
// the whole cost. OWASP's current floor for this construction is 600,000.
const DefaultIterations = 600_000

const (
	algLabel   = "pbkdf2-sha256"
	saltLength = 16
	keyLength  = 32
)

var (
	ErrInvalidHash = errors.New("pwhash: stored hash is not a recognised pbkdf2-sha256 encoding")
	ErrMismatch    = errors.New("pwhash: password does not match")
)

// Hash derives a new salted hash for password.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("pwhash: read salt: %w", err)
	}
	key := pbkdf2(password, salt, DefaultIterations, keyLength)

	return fmt.Sprintf("%s$%d$%s$%s",
		algLabel,
		DefaultIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify reports whether password matches encoded. It returns ErrMismatch
// for a wrong password and ErrInvalidHash for an unreadable stored value —
// the two are different problems and the caller should not conflate them.
func Verify(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != algLabel {
		return ErrInvalidHash
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return ErrInvalidHash
	}

	got := pbkdf2(password, salt, iterations, len(want))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}

// pbkdf2 implements PBKDF2 with HMAC-SHA256 as the PRF (RFC 8018 §5.2).
//
//	DK   = T_1 || T_2 || … || T_n
//	T_i  = U_1 xor U_2 xor … xor U_c
//	U_1  = PRF(password, salt || INT_32_BE(i))
//	U_j  = PRF(password, U_{j-1})
func pbkdf2(password string, salt []byte, iterations, keyLen int) []byte {
	prf := hmac.New(sha256.New, []byte(password))
	hashLen := prf.Size()
	blocks := (keyLen + hashLen - 1) / hashLen

	dk := make([]byte, 0, blocks*hashLen)
	block := make([]byte, 4)
	u := make([]byte, hashLen)
	t := make([]byte, hashLen)

	for i := 1; i <= blocks; i++ {
		binary.BigEndian.PutUint32(block, uint32(i))

		prf.Reset()
		prf.Write(salt)
		prf.Write(block)
		u = prf.Sum(u[:0])
		copy(t, u)

		for j := 1; j < iterations; j++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for k := range t {
				t[k] ^= u[k]
			}
		}
		dk = append(dk, t...)
	}

	return dk[:keyLen]
}
