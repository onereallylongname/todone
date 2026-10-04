package store

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID returns a short, locally-unique hex id (10 hex chars from 5 random
// bytes). Stdlib-only — no uuid dependency for something that only needs to
// avoid collisions within one person's task list.
func NewID() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is effectively unheard of on supported
		// platforms; fall back to a fixed-looking id rather than panicking.
		return "00000000"
	}
	return hex.EncodeToString(b[:])
}
