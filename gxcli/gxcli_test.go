package gxcli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

func TestREQ_AUT_17_FmtCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Card.gx")
	unformatted := []byte("package card\n\n\n<p>x</p>\n")
	if err := os.WriteFile(path, unformatted, 0o644); err != nil {
		t.Fatal(err)
	}

	if code := gxcli.Main([]string{"fmt", "--check", path}); code == 0 {
		t.Fatal("--check accepted an unformatted file")
	}

	if code := gxcli.Main([]string{"fmt", path}); code != 0 {
		t.Fatalf("fmt exit code = %d", code)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(unformatted) {
		t.Fatal("fmt did not rewrite the file")
	}
	if code := gxcli.Main([]string{"fmt", path}); code != 0 {
		t.Fatalf("second fmt exit code = %d", code)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("fmt is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if code := gxcli.Main([]string{"fmt", "--check", path}); code != 0 {
		t.Fatalf("--check rejected a formatted file: exit %d", code)
	}

	if code := gxcli.Main([]string{"fmt", filepath.Join(dir, "Missing.gx")}); code == 0 {
		t.Fatal("fmt accepted a missing file")
	}
	if code := gxcli.Main([]string{"frobnicate"}); code == 0 {
		t.Fatal("unknown command exited zero")
	}
}
