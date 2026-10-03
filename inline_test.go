package gx_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// FuzzSI_05_InlinedValue checks that a server value inlined into a client
// expression cannot leave its attribute (SI-05).
func FuzzSI_05_InlinedValue(f *testing.F) {
	f.Add(`x"><script>alert(1)</script>`)
	f.Add("a'b&c<d>")
	f.Add(`";alert(1);//`)
	f.Add("line1\nline2")
	f.Fuzz(func(t *testing.T, s string) {
		// This is the shape the compiler emits for an inlined value:
		// gx.JSON(<value>) inside the adapter expression.
		node := gx.El("div", gx.Attrs{
			{Key: "data-on:click", Value: `@post('/go?q=` + gx.JSON(s) + `')`},
		})
		out := gx.String(node)
		const prefix = `<div data-on:click="`
		const suffix = `"></div>`
		if !strings.HasPrefix(out, prefix) || !strings.HasSuffix(out, suffix) {
			t.Fatalf("output = %q", out)
		}
		value := out[len(prefix) : len(out)-len(suffix)]
		if strings.ContainsAny(value, `"<>`) {
			t.Fatalf("inlined value %q left the attribute: %q", s, out)
		}
		// The JSON form round-trips, and equals encoding/json for the
		// same input.
		want, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if gx.JSON(s) != string(want) {
			t.Fatalf("JSON(%q) = %q, want %q", s, gx.JSON(s), want)
		}
		var back string
		if err := json.Unmarshal([]byte(gx.JSON(s)), &back); err != nil {
			t.Fatalf("JSON(%q) = %q: %v", s, gx.JSON(s), err)
		}
	})
}
