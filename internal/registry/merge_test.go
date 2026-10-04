package registry_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/registry"
)

// TestREQ_REG_03_Merge covers the three-way merge of gx update: clean,
// local-only, upstream-only and conflict (REQ-REG-03).
func TestREQ_REG_03_Merge(t *testing.T) {
	base := "one\ntwo\nthree\nfour\nfive\n"
	tests := []struct {
		name     string
		local    string
		upstream string
		want     string
		conflict bool
	}{
		{
			name:     "clean",
			local:    base,
			upstream: base,
			want:     base,
		},
		{
			name:     "local-only",
			local:    "one\nTWO\nthree\nfour\nfive\n",
			upstream: base,
			want:     "one\nTWO\nthree\nfour\nfive\n",
		},
		{
			name:     "upstream-only",
			local:    base,
			upstream: "one\ntwo\nthree\nFOUR\nfive\n",
			want:     "one\ntwo\nthree\nFOUR\nfive\n",
		},
		{
			name:     "both-different-lines",
			local:    "one\nTWO\nthree\nfour\nfive\n",
			upstream: "one\ntwo\nthree\nFOUR\nfive\n",
			want:     "one\nTWO\nthree\nFOUR\nfive\n",
		},
		{
			name:     "conflict",
			local:    "one\nLOCAL\nthree\nfour\nfive\n",
			upstream: "one\nUPSTREAM\nthree\nfour\nfive\n",
			conflict: true,
			want: "one\n<<<<<<< local\nLOCAL\n=======\nUPSTREAM\n>>>>>>> upstream\nthree\nfour\nfive\n",
		},
		{
			name:     "same-edit",
			local:    "one\nSAME\nthree\nfour\nfive\n",
			upstream: "one\nSAME\nthree\nfour\nfive\n",
			want:     "one\nSAME\nthree\nfour\nfive\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conflict := registry.Merge(base, tt.local, tt.upstream)
			if conflict != tt.conflict {
				t.Fatalf("conflict = %v, want %v:\n%s", conflict, tt.conflict, got)
			}
			if got != tt.want {
				t.Fatalf("merged =\n%s\nwant:\n%s", got, tt.want)
			}
			if tt.conflict && !strings.Contains(got, "<<<<<<< local") {
				t.Fatalf("conflict wrote no markers:\n%s", got)
			}
		})
	}
}
