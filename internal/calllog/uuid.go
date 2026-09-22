package calllog

import (
	"crypto/rand"
	"fmt"
)

// newUUIDv4 generates a random (v4) UUID using only crypto/rand.
//
// Laravel's HasUuids trait does the same thing (Str::uuid(), backed by
// ramsey/uuid's v4 generator) — this is not a simplification, RFC 4122 v4
// is nothing but 16 random bytes with two fixed bits, so there is no
// library-shaped gap between this and what Laravel does. Needed because
// go.sum is not writable in this repo (see internal/security/jwtsig's own
// doc comment for the same constraint), so google/uuid is not available.
func newUUIDv4() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is effectively unrecoverable.
		panic("calllog: failed to read random bytes for uuid: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
