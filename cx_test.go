package gx_test

import (
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
		{"shadow color", []string{"shadow-lg", "shadow-red-500"}, "shadow-red-500"},
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
