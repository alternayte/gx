package gx_test

import (
	"os"
	"strings"
	"testing"
)

func readRuntimeJS(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("runtime/js/gx.js")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestREQ_STY_09_NavigationTransitionRuntime covers the runtime half of
// layout-aware view transitions: navigation runs inside
// document.startViewTransition (REQ-STY-09).
func TestREQ_STY_09_NavigationTransitionRuntime(t *testing.T) {
	js := readRuntimeJS(t)
	if !strings.Contains(js, "startViewTransition") {
		t.Fatal("the runtime does not start a view transition")
	}
	if !strings.Contains(js, "Gx-Nav") {
		t.Fatal("the runtime no longer performs layout-aware navigation")
	}
}

// TestREQ_STY_10_ReducedMotionRuntime covers the reduced-motion default: the
// runtime skips the transition and the injected CSS stops the transition
// animations (REQ-STY-10).
func TestREQ_STY_10_ReducedMotionRuntime(t *testing.T) {
	js := readRuntimeJS(t)
	for _, want := range []string{"prefers-reduced-motion", "animation: none"} {
		if !strings.Contains(js, want) {
			t.Fatalf("the runtime lacks %q", want)
		}
	}
}
