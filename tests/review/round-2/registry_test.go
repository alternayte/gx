package round2_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/registry"
)

// publish writes one item of a registry directory.
func publish(t *testing.T, dir, version string, files map[string]string) {
	t.Helper()
	item := registry.Item{
		Name:        "widget",
		Version:     version,
		Description: "A widget.",
		Kind:        "component",
		Usage:       "# widget\n",
	}
	for _, target := range []string{"ui/widget/Widget.gx", "ui/widget/styles.go"} {
		content, ok := files[target]
		if !ok {
			continue
		}
		sum := sha256.Sum256([]byte(content))
		item.Files = append(item.Files, registry.File{
			Path: filepath.Base(target), Target: target, Content: content, SHA256: hex.EncodeToString(sum[:]),
		})
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "items", "widget.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestREQ_REG_03_UpdateDoesNotOverwriteALocalFileSilently: the app owns the
// installed directory. The developer adds a file there, and a later
// registry version adds a file with the same target. gx update classifies
// the target as "upstream-added" without a look at the app file, writes the
// upstream text over it and reports no conflict. REQ-REG-03 says: "A
// conflict writes markers and is reported. Nothing is overwritten
// silently." gx add refuses the same case.
func TestREQ_REG_03_UpdateDoesNotOverwriteALocalFileSilently(t *testing.T) {
	source, root := t.TempDir(), t.TempDir()
	widget := "package widget\n\n<div>widget</div>\n"
	publish(t, source, "0.1.0", map[string]string{"ui/widget/Widget.gx": widget})
	in := &registry.Installer{Root: root, Source: source}
	if _, err := in.Add("widget", ""); err != nil {
		t.Fatalf("Add: %v", err)
	}

	local := "package widget\n\n// localWork is the work of the developer.\nconst localWork = \"keep me\"\n"
	path := filepath.Join(root, "ui", "widget", "styles.go")
	if err := os.WriteFile(path, []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	publish(t, source, "0.2.0", map[string]string{
		"ui/widget/Widget.gx": widget,
		"ui/widget/styles.go": "package widget\n\nconst upstream = \"new file of 0.2.0\"\n",
	})

	changes, conflict, err := in.Update("widget", "")
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("the local file is gone: %v", readErr)
	}
	kept := strings.Contains(string(after), `const localWork = "keep me"`)
	if err != nil {
		// A refusal is a report. The file must stay.
		if !kept {
			t.Fatalf("Update failed with %v and the local text is lost:\n%s", err, after)
		}
		return
	}
	if !kept || !conflict {
		t.Fatalf("gx update overwrote a local file silently: conflict = %v, changes = %+v, local text kept = %v\nfile now:\n%s", conflict, changes, kept, after)
	}
}

// TestREQ_REG_03_UpdateKeepsLocalEditsNearAnUpstreamChange: the three-way
// merge walks the base lines and takes a hunk only when it starts at the
// current line (internal/registry/merge.go, hunkAt). An upstream hunk that
// starts before a local hunk and ends after it moves the walk past the
// start of the local hunk. That local hunk and every later local hunk are
// then dropped: the app file gets the upstream text, with no marker and no
// conflict report. REQ-REG-03 says: "A conflict writes markers and is
// reported. Nothing is overwritten silently."
//
// The developer changed line c and line g. The upstream changed lines b, c
// and d. Line c is a conflict. Line g is a clean local change.
func TestREQ_REG_03_UpdateKeepsLocalEditsNearAnUpstreamChange(t *testing.T) {
	source, root := t.TempDir(), t.TempDir()
	target := "ui/widget/Widget.gx"
	publish(t, source, "0.1.0", map[string]string{target: "a\nb\nc\nd\ne\nf\ng\n"})
	in := &registry.Installer{Root: root, Source: source}
	if _, err := in.Add("widget", ""); err != nil {
		t.Fatalf("Add: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(target))
	if err := os.WriteFile(path, []byte("a\nb\nC-local\nd\ne\nf\nG-local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	publish(t, source, "0.2.0", map[string]string{target: "a\nB-up\nC-up\nD-up\ne\nf\ng\n"})

	changes, conflict, err := in.Update("widget", "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	after := string(raw)
	for _, want := range []string{"G-local", "C-local", "C-up"} {
		if !strings.Contains(after, want) {
			t.Errorf("the merged file lacks %q", want)
		}
	}
	if !conflict {
		t.Errorf("both sides changed line c, and Update reports no conflict")
	}
	if t.Failed() {
		t.Logf("changes = %+v\nfile now:\n%s", changes, after)
	}
}
