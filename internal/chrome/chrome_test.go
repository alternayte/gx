package chrome_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/alternayte/gx/internal/chrome"
)

// TestREQ_AI_04_AxePin covers the vendored axe-core: the embedded file is
// the pinned release, so an audit needs no node and no download
// (REQ-AI-04).
func TestREQ_AI_04_AxePin(t *testing.T) {
	data, err := os.ReadFile("axe.min.js")
	if err != nil {
		t.Fatal(err)
	}
	if got := chrome.SumAxe(data); got != chrome.AxeSHA256 {
		t.Fatalf("axe.min.js has hash %s, want the pinned %s", got, chrome.AxeSHA256)
	}
	if !bytes.Contains(data[:200], []byte("axe v"+chrome.AxeVersion)) {
		t.Fatalf("axe.min.js is not version %s", chrome.AxeVersion)
	}
	if _, err := os.Stat("axe.LICENSE"); err != nil {
		t.Fatal("the axe-core licence is missing")
	}
}
