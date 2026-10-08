package gx

import (
	"sync/atomic"

	"github.com/alternayte/gx/internal/secretscan"
)

// Secret is a value that must not leave the server (SI-04). It renders and
// marshals as "[redacted]".
type Secret string

// String returns the redacted form.
func (s Secret) String() string { return "[redacted]" }

// Reveal returns the secret value. Use it on the server only.
func (s Secret) Reveal() string { return string(s) }

// MarshalJSON never writes the secret.
func (s Secret) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }

// hasSecretKey reports whether a value holds a map with a Secret as its key
// type. encoding/json writes the key of a map as its text and calls no
// method of the key, so MarshalJSON cannot redact it (SI-04).
func hasSecretKey(v any) bool { return secretscan.HasKeyOfType(v, Secret("")) }

var devMode atomic.Bool

// SetDev turns the dev checks on or off. gx dev sets it (REQ-DEV-01).
func SetDev(on bool) { devMode.Store(on) }

// IsDev reports whether the dev checks are on.
func IsDev() bool { return devMode.Load() }

// checkSecret panics in dev when a secret reaches a client expression
// (SI-04).
func checkSecret(v any) {
	if !devMode.Load() {
		return
	}
	switch v.(type) {
	case Secret, *Secret:
		panic("gx: gx.Secret cannot enter a client expression; use Reveal() on the server")
	}
}
