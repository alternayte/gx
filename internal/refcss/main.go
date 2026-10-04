// Command refcss builds the dev-only reference stylesheet of the visual
// parity suite (REQ-REG-08) with the pinned Tailwind binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alternayte/gx/internal/tailwind"
)

func main() {
	in := flag.String("in", "", "input CSS file")
	out := flag.String("out", "", "output CSS file")
	root := flag.String("root", "", "module root (default: repo root)")
	flag.Parse()
	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "refcss: -in and -out are required")
		os.Exit(2)
	}
	r := *root
	if r == "" {
		wd, err := os.Getwd()
		if err != nil {
			fatal(err)
		}
		r = wd
	}
	m := &tailwind.Manager{Root: r}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err)
	}
	if _, err := m.Run(context.Background(), "-i", *in, "-o", *out, "--minify"); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "refcss:", err)
	os.Exit(1)
}
