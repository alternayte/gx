package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const sharedNote = `package notes

props {
  Room gx.Room
}

signals {
  Draft string = ""
  // Typing is true while a viewer writes.
  shared Typing bool = false
  shared Title string = "New"
}

<div>
  <input bind:value={$Title} />
  <span show={$Typing}>A viewer writes</span>
</div>
`

// TestREQ_ACT_21_SharedSignalCodegen checks the generated code of a
// component with shared signals: the root has the signed room key, the
// address of the room and the effect that tells the runtime each change; the
// file has the routes of the room for the app to mount, with the Signals
// struct and only the shared names (REQ-ACT-21).
func TestREQ_ACT_21_SharedSignalCodegen(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"notes/Note.gx": sharedNote,
	})
	src := string(generateFiles(t, dir)[filepath.Join(dir, "notes/Note_gx.go")])
	for _, want := range []string{
		"Draft string `json:\"draft\"`",
		"Typing bool   `json:\"typing\"`",
		`gx.Attr{Key: "data-gx-room", Value: string(p.Room), Kind: gx.AttrText}`,
		`gx.Attr{Key: "data-gx-room-url", Value: gx.RoomURL("notes.Note"), Kind: gx.AttrURL}`,
		`gx.Attr{Key: "data-effect", Value: gx.ShareEffect("notes.Note", p.GxKey, "typing", "title"), Kind: gx.AttrText}`,
		`var NoteRoom = gx.SharedSignals[NoteSignals]("notes.Note", "typing", "title")`,
		"func (s *NoteSignals) GxFieldName(ptr any) string {",
		"case &s.Title:\n\t\treturn \"title\"",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Note_gx.go lacks %s", want)
		}
	}
	if t.Failed() {
		t.Logf("Note_gx.go:\n%s", src)
	}
}

// TestREQ_ACT_21_SharedSignalNeedsARoom checks GX4014: a shared signal in a
// component with no prop of type gx.Room (REQ-ACT-21).
func TestREQ_ACT_21_SharedSignalNeedsARoom(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"notes/Note.gx": strings.Replace(sharedNote, "props {\n  Room gx.Room\n}\n\n", "", 1),
	})
	_, diags := compiler.Generate(dir)
	if len(diags) != 1 || diags[0].Code != compiler.CodeSharedSignal || diags[0].Line != 6 || !strings.Contains(diags[0].Msg, `"Typing"`) {
		t.Fatalf("diagnostics = %v, want GX4014 at the first shared signal", diags)
	}
}

// TestREQ_ACT_21_FormatKeepsShared checks that gx fmt keeps the word of a
// shared signal and gives one form (REQ-AUT-17).
func TestREQ_ACT_21_FormatKeepsShared(t *testing.T) {
	messy := strings.Replace(sharedNote, "  shared Typing bool = false\n", "  shared   Typing   bool=false\n", 1)
	out, diags := compiler.FormatSource("Note.gx", []byte(messy))
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	for _, want := range []string{"  Draft         string = \"\"\n", "  shared Typing bool = false\n", "  shared Title  string = \"New\"\n"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the formatted file lacks %q:\n%s", want, out)
		}
	}
	again, diags := compiler.FormatSource("Note.gx", out)
	if len(diags) > 0 || string(again) != string(out) {
		t.Errorf("a second format changed the file: %v\n%s", diags, again)
	}
}
