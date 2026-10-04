package gx_test

import (
	"bytes"
	"compress/gzip"
	"os"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// TestREQ_REG_07_BehaviorBudget checks the 4 KB gzipped budget of the
// component behaviour runtime (REQ-REG-07).
func TestREQ_REG_07_BehaviorBudget(t *testing.T) {
	data, err := os.ReadFile("runtime/js/behavior.js")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	const budget = 4 * 1024
	if buf.Len() > budget {
		t.Fatalf("behaviour runtime = %d bytes gzipped, budget %d", buf.Len(), budget)
	}
	t.Logf("behaviour runtime: %d bytes gzipped (budget %d)", buf.Len(), budget)
}

// TestREQ_REG_07_BehaviorLoads checks that the behaviour runtime joins only
// a page whose markup uses a behaviour marker (REQ-REG-07).
func TestREQ_REG_07_BehaviorLoads(t *testing.T) {
	body := servePage(t, &nfr04Adapter{}, func() gx.Node {
		return gx.El("div", gx.Attrs{
			{Key: "data-gx-roving", Value: ""},
			{Key: "data-gx-dismiss", Value: ""},
		})
	})
	if !strings.Contains(body, "/_gx/behavior.js") {
		t.Fatalf("behaviour page lacks the behaviour runtime:\n%s", body)
	}
	if strings.Contains(body, "/_gx/gx.js") {
		t.Fatalf("behaviour page ships the core runtime:\n%s", body)
	}
	if strings.Contains(body, "/_gx/nfr04-adapter.js") {
		t.Fatalf("behaviour page ships the adapter runtime:\n%s", body)
	}

	plain := servePage(t, &nfr04Adapter{}, func() gx.Node {
		return gx.El("p", nil, gx.Text("plain"))
	})
	if strings.Contains(plain, "/_gx/behavior.js") {
		t.Fatalf("plain page ships the behaviour runtime:\n%s", plain)
	}
}
