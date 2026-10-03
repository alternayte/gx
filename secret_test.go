package gx_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// TestSI_04_SecretRedacted checks a secret renders and marshals redacted.
func TestSI_04_SecretRedacted(t *testing.T) {
	s := gx.Secret("hunter2")
	if got := gx.String(gx.Value(s)); got != "[redacted]" {
		t.Fatalf("rendered secret = %q", got)
	}
	if got := gx.JSON(s); got != `"[redacted]"` {
		t.Fatalf("JSON secret = %s", got)
	}
	if strings.Contains(gx.JSON(struct{ S gx.Secret }{s}), "hunter2") {
		t.Fatalf("secret leaked through a struct: %s", gx.JSON(struct{ S gx.Secret }{s}))
	}
	if s.Reveal() != "hunter2" {
		t.Fatal("Reveal did not return the value")
	}
}

// TestSI_04_DevPanic checks the dev build fails loudly on a leak attempt.
func TestSI_04_DevPanic(t *testing.T) {
	gx.SetDev(true)
	defer gx.SetDev(false)
	defer func() {
		if recover() == nil {
			t.Fatal("JSON did not panic on a secret in dev")
		}
	}()
	_ = gx.JSON(gx.Secret("hunter2"))
}
