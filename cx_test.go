package gx_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/alternayte/gx"
)

// TestREQ_STY_04_Cx covers gx.Cx: later classes win inside one conflict
// group, modifiers scope the conflict and unknown classes stay (REQ-STY-04).
func TestREQ_STY_04_Cx(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parts []string
		want  string
	}{
		{"same group", []string{"p-4", "p-2"}, "p-2"},
		{"later sub group keeps both", []string{"p-4", "px-2"}, "p-4 px-2"},
		{"parent removes child", []string{"px-2", "p-4"}, "p-4"},
		{"ordered parts", []string{"p-4 p-2", "p-1"}, "p-1"},
		{"modifier scopes", []string{"hover:p-4", "p-2"}, "hover:p-4 p-2"},
		{"same modifier", []string{"hover:p-4", "hover:p-2"}, "hover:p-2"},
		{"text size and color", []string{"text-red-500", "text-lg"}, "text-red-500 text-lg"},
		{"text size conflict", []string{"text-sm", "text-lg"}, "text-lg"},
		{"text color conflict", []string{"text-red-500", "text-blue-500"}, "text-blue-500"},
		{"font family and weight", []string{"font-sans", "font-bold"}, "font-sans font-bold"},
		{"rounded child after parent", []string{"rounded", "rounded-t-lg"}, "rounded rounded-t-lg"},
		{"rounded parent removes child", []string{"rounded-t-lg", "rounded"}, "rounded"},
		{"display conflict", []string{"block", "hidden", "block"}, "block"},
		{"flex direction", []string{"flex-row", "flex-col"}, "flex-col"},
		{"arbitrary value", []string{"p-[3px]", "p-4"}, "p-4"},
		{"negative", []string{"-mt-4", "-mt-2"}, "-mt-2"},
		{"important", []string{"!p-4", "!p-2"}, "!p-2"},
		{"margin sides", []string{"mx-2", "m-4"}, "m-4"},
		{"unknown classes stay", []string{"my-thing", "another"}, "my-thing another"},
		{"empty", []string{"", "  "}, ""},
		{"inset child after", []string{"top-2", "inset-0"}, "inset-0"},
		{"inset parent then child", []string{"inset-0", "top-2"}, "inset-0 top-2"},
		{"shadow size and color", []string{"shadow-lg", "shadow-red-500"}, "shadow-lg shadow-red-500"},
		{"shadow size conflict", []string{"shadow-lg", "shadow-xs"}, "shadow-xs"},
		{"text align and color", []string{"text-left", "text-muted-foreground"}, "text-left text-muted-foreground"},
		{"text align conflict", []string{"text-left", "text-center"}, "text-center"},
		{"text wrap and color", []string{"text-balance", "text-foreground"}, "text-balance text-foreground"},
		{"text size with leading", []string{"text-sm/relaxed", "text-muted-foreground"}, "text-sm/relaxed text-muted-foreground"},
		{"border side and color", []string{"border-b", "border-border"}, "border-b border-border"},
		{"border side width conflict", []string{"border-b", "border-b-2"}, "border-b-2"},
		{"border removes side", []string{"border-b", "border"}, "border"},
		{"ring arbitrary width", []string{"ring-[3px]", "ring-0"}, "ring-0"},
		{"ring width and color", []string{"ring-2", "ring-ring/50"}, "ring-2 ring-ring/50"},
		{"size removes width and height", []string{"h-4 w-4", "size-6"}, "size-6"},
		{"important keeps plain", []string{"p-4!", "p-2"}, "p-4! p-2"},
		{"important suffix conflict", []string{"m-0!", "m-2!"}, "m-2!"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := gx.Cx(tc.parts...); got != tc.want {
				t.Fatalf("Cx(%q) = %q, want %q", tc.parts, got, tc.want)
			}
		})
	}
}

// BenchmarkCx10 measures the 10-class merge budget of REQ-STY-04.
func BenchmarkCx10(b *testing.B) {
	parts := []string{
		"flex", "items-center", "gap-4", "p-4", "rounded-lg",
		"bg-white", "text-sm", "font-medium", "hover:bg-gray-50", "p-2",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = gx.Cx(parts...)
	}
}

// TestREQ_STY_04_TailwindMergeSuite runs the test suite of tailwind-merge
// 3.7.0 against gx.Cx: every expectation of the default configuration in
// its test files and every example of its documentation (REQ-STY-04).
// tools/twmerge/extract.ts writes the cases from the upstream files.
func TestREQ_STY_04_TailwindMergeSuite(t *testing.T) {
	raw, err := os.ReadFile("testdata/tailwind-merge-3.7.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Cases []struct {
			File string   `json:"file"`
			Test string   `json:"test"`
			Args []string `json:"args"`
			Want string   `json:"want"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatal(err)
	}
	files := map[string]int{}
	wrong := 0
	for _, c := range suite.Cases {
		files[c.File]++
		if got := gx.Cx(c.Args...); got != c.Want {
			wrong++
			if wrong <= 20 {
				t.Errorf("%s: %s\n  Cx(%q)\n   = %q\nwant %q", c.File, c.Test, c.Args, got, c.Want)
			}
		}
	}
	if wrong > 0 {
		t.Errorf("%d of %d cases fail", wrong, len(suite.Cases))
	}
	// The port covers the whole suite, not a part of it.
	if len(suite.Cases) < 440 || len(files) < 19 {
		t.Fatalf("the suite holds %d cases of %d files; the upstream suite gives 442 of 19", len(suite.Cases), len(files))
	}
}

// TestREQ_STY_04_AllocationLight covers the allocation side of gx.Cx: a
// merge of ten classes makes one allocation, the result, and a single
// class or a merged string with no conflict makes none (REQ-STY-04).
func TestREQ_STY_04_AllocationLight(t *testing.T) {
	parts := []string{
		"flex", "items-center", "gap-4", "p-4", "rounded-lg",
		"bg-white", "text-sm", "font-medium", "hover:bg-gray-50", "p-2",
	}
	if n := testing.AllocsPerRun(200, func() { _ = gx.Cx(parts...) }); n > 1 {
		t.Fatalf("a merge of ten classes makes %v allocations, want at most 1", n)
	}
	for _, one := range []string{"p-4", "flex items-center gap-4 hover:bg-gray-50"} {
		if n := testing.AllocsPerRun(200, func() { _ = gx.Cx(one) }); n != 0 {
			t.Fatalf("Cx(%q) makes %v allocations, want 0", one, n)
		}
	}
	// The same input gives the same output.
	first := gx.Cx(parts...)
	for i := 0; i < 50; i++ {
		if got := gx.Cx(parts...); got != first {
			t.Fatalf("run %d = %q, first = %q", i, got, first)
		}
	}
	if first != "flex items-center gap-4 rounded-lg bg-white text-sm font-medium hover:bg-gray-50 p-2" {
		t.Fatalf("Cx = %q", first)
	}
}
