// Command docsgen writes the component pages of the docs site from the
// registry (REQ-DOC-02). It reads every item under registry/ and writes one
// content page per item, the components index and the Go list of examples
// under docs/. The output is deterministic and committed.
//
//	go run ./internal/docsgen           write the files
//	go run ./internal/docsgen --check   fail when a file is stale
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	root := flag.String("root", ".", "the repository root")
	check := flag.Bool("check", false, "fail when the committed output is stale; write nothing")
	verbose := flag.Bool("v", false, "list every fixture that falls back to an expression")
	flag.Parse()
	if err := run(*root, *check, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run generates the docs files of the repository at root.
func run(root string, check, verbose bool) error {
	reg, err := loadRegistry(root)
	if err != nil {
		return err
	}
	pages := buildPages(reg)
	files, err := render(reg, pages)
	if err != nil {
		return err
	}
	stale, err := staleFiles(root, files)
	if err != nil {
		return err
	}
	if check {
		if len(stale) > 0 {
			return fmt.Errorf("docsgen: stale: %s", strings.Join(stale, ", "))
		}
		return nil
	}
	for _, path := range stale {
		full := filepath.Join(root, filepath.FromSlash(path))
		data, ok := files[path]
		if !ok {
			if err := os.Remove(full); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
	}
	report(pages, len(stale), verbose)
	return nil
}

// staleFiles returns the output paths whose file differs from the
// generated content, and the item pages no item owns any more.
func staleFiles(root string, files map[string][]byte) ([]string, error) {
	var stale []string
	for path, want := range files {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err != nil || !bytes.Equal(got, want) {
			stale = append(stale, path)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(pagesDir)))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		path := pagesDir + "/" + e.Name()
		if _, ok := files[path]; !ok && !e.IsDir() {
			stale = append(stale, path)
		}
	}
	sort.Strings(stale)
	return stale, nil
}

// report prints the item, example and converter counts.
func report(pages []*page, written int, verbose bool) {
	examples, tags, fallbacks := 0, 0, 0
	for _, p := range pages {
		examples += len(p.Examples)
		for _, ex := range p.Examples {
			for _, part := range ex.Parts {
				if part.Snippet.Tag {
					tags++
					continue
				}
				fallbacks++
				if verbose {
					fmt.Printf("fallback: %s %s/%s: %s\n", p.Item.Name, part.Comp.Name, part.Fixture.Name, part.Snippet.Why)
				}
			}
		}
	}
	fmt.Printf("docsgen: %d items, %d examples, %d fixtures as tags, %d as fallback expressions, %d files written\n",
		len(pages), examples, tags, fallbacks, written)
}
