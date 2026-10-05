// Command stelint checks Markdown files against the rules of ASD-STE100
// that a program can decide (NFR-10). `just ste-lint` runs it on the docs.
//
//	go run ./internal/stelint/cmd/stelint [--words <file>] <file or directory> ...
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/alternayte/gx/internal/stelint"
)

func main() {
	wordFile := flag.String("words", "scripts/ste-words.txt", "the list of words that are not approved")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: stelint [--words <file>] <file or directory> ...")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*wordFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stelint:", err)
		os.Exit(1)
	}
	findings, files, err := stelint.LintPaths(flag.Args(), stelint.ParseWords(string(raw)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "stelint:", err)
		os.Exit(1)
	}
	for _, f := range findings {
		fmt.Println(f)
	}
	if len(findings) > 0 {
		fmt.Fprintf(os.Stderr, "stelint: %d findings in %d files\n", len(findings), files)
		os.Exit(1)
	}
	fmt.Printf("stelint: %d files follow the rules\n", files)
}
