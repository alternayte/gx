// Command evidence writes and checks docs/build/evidence.json.
//
// --write runs the Go tests once and records each covering test result. The
// browser suites are not run again: their results come from the reports that
// tests/e2e/record.sh left, and only from a run on a clean tree at HEAD.
// --check fails when the file is not for HEAD, or a PASS id lacks a passing
// test or has a failed one.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var idRe = regexp.MustCompile(`(REQ-[A-Z]+-[0-9]+|NFR-[0-9]+|SI-[0-9]+)`)

var tsTitleRe = regexp.MustCompile("(?:\\btest|\\bit)\\s*\\(\\s*[\"'`]([^\"'`]+)")

type ledgerRow struct {
	ID        string
	Release   string
	Milestone string
	Status    string
	Tests     []string
	Commit    string
}

type testResult struct {
	Name   string `json:"name"`
	Result string `json:"result"`
}

type idEvidence struct {
	ID     string       `json:"id"`
	Status string       `json:"status"`
	Tests  []testResult `json:"tests"`
}

type evidence struct {
	Commit      string            `json:"commit"`
	GeneratedAt string            `json:"generated_at"`
	IDs         []idEvidence      `json:"ids"`
	Benchmarks  map[string]string `json:"benchmarks"`
	SizeBudgets map[string]string `json:"size_budgets"`
}

func main() {
	write := flag.Bool("write", false, "write docs/build/evidence.json")
	check := flag.Bool("check", false, "check docs/build/evidence.json")
	dir := flag.String("dir", filepath.Join("docs", "build"), "directory of ledger.md and evidence.json")
	flag.Parse()
	if *write == *check {
		fatal("exactly one of --write or --check is required")
	}
	root, err := repoRoot()
	if err != nil {
		fatal(err.Error())
	}
	if *write {
		if err := writeEvidence(root, *dir); err != nil {
			fatal(err.Error())
		}
		return
	}
	if err := checkEvidence(root, *dir); err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "evidence: "+msg)
	os.Exit(1)
}

func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("find repository root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func headCommit(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("find HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func readLedger(dir string) ([]ledgerRow, error) {
	data, err := os.ReadFile(filepath.Join(dir, "ledger.md"))
	if err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	var rows []ledgerRow
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 6 {
			continue
		}
		id := strings.TrimSpace(cells[1])
		if !idRe.MatchString(id) || idRe.FindString(id) != id {
			continue
		}
		row := ledgerRow{
			ID:        id,
			Release:   strings.TrimSpace(cells[2]),
			Milestone: strings.TrimSpace(cells[3]),
			Status:    strings.TrimSpace(cells[4]),
		}
		for _, t := range strings.Split(strings.TrimSpace(cells[5]), ",") {
			if t = strings.TrimSpace(t); t != "" {
				row.Tests = append(row.Tests, t)
			}
		}
		if len(cells) > 6 {
			row.Commit = strings.TrimSpace(cells[6])
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, errors.New("ledger has no ID rows")
	}
	return rows, nil
}

type testScan struct {
	byID  map[string][]string
	names map[string]bool
}

func scanTests(root string) (*testScan, error) {
	scan := &testScan{byID: map[string][]string{}, names: map[string]bool{}}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "docs", "dist", "bin":
				return fs.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(d.Name(), "_test.go"):
			names, err := goTestNames(path)
			if err != nil {
				return err
			}
			for _, name := range names {
				scan.names[name] = true
				for _, id := range idRe.FindAllString(strings.ReplaceAll(name, "_", "-"), -1) {
					scan.byID[id] = appendUnique(scan.byID[id], name)
				}
			}
		case strings.HasSuffix(d.Name(), ".ts"), strings.HasSuffix(d.Name(), ".tsx"):
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range tsTitleRe.FindAllStringSubmatch(string(data), -1) {
				for _, id := range idRe.FindAllString(m[1], -1) {
					scan.byID[id] = appendUnique(scan.byID[id], m[1])
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return scan, nil
}

func goTestNames(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var names []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		name := fn.Name.Name
		if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") ||
			strings.HasPrefix(name, "Example") || strings.HasPrefix(name, "Fuzz") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// quietPackages hold timing budgets that are defined on a quiet machine
// (SDD section 13.1). `just test` runs them first and alone, and so does the
// evidence run: their result comes from the solo run, not from the full run
// where every package competes for the processor.
var quietPackages = []string{
	"github.com/alternayte/gx/internal/devserver",
	"github.com/alternayte/gx/internal/lsp",
	"github.com/alternayte/gx/internal/exporter",
}

func runGoTests(root string) (map[string]string, error) {
	results := map[string]string{}
	quiet := map[string]bool{}
	for _, pkg := range quietPackages {
		quiet[pkg] = true
		out, err := goTestJSON(root, pkg)
		if err != nil {
			return nil, err
		}
		if err := mergeResults(results, out, nil); err != nil {
			return nil, err
		}
	}
	out, err := goTestJSON(root, "./...")
	if err != nil {
		return nil, err
	}
	if err := mergeResults(results, out, quiet); err != nil {
		return nil, err
	}
	return results, nil
}

// goTestJSON runs `go test -json` on one pattern. A failing test is a
// result, not an error.
func goTestJSON(root, pattern string) ([]byte, error) {
	cmd := exec.Command("go", "test", "-json", pattern)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return nil, fmt.Errorf("run go test: %w", err)
		}
	}
	return out, nil
}

// mergeResults adds the top-level test results of one `go test -json`
// stream to results. It leaves out the packages in skip. A failure is never
// replaced by a pass.
func mergeResults(results map[string]string, stream []byte, skip map[string]bool) error {
	dec := json.NewDecoder(bytes.NewReader(stream))
	for {
		var ev struct {
			Action  string
			Package string
			Test    string
		}
		if err := dec.Decode(&ev); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decode go test output: %w", err)
		}
		if ev.Test == "" || strings.Contains(ev.Test, "/") || skip[ev.Package] {
			continue
		}
		switch ev.Action {
		case "pass", "fail", "skip":
			if results[ev.Test] != "fail" {
				results[ev.Test] = ev.Action
			}
		}
	}
}

func writeEvidence(root, dir string) error {
	rows, err := readLedger(dir)
	if err != nil {
		return err
	}
	scan, err := scanTests(root)
	if err != nil {
		return err
	}
	results, err := runGoTests(root)
	if err != nil {
		return err
	}
	head, err := headCommit(root)
	if err != nil {
		return err
	}
	browser, ignored, err := readBrowserResults(filepath.Join(root, browserResultsDir), head)
	if err != nil {
		return err
	}
	for _, msg := range ignored {
		fmt.Fprintln(os.Stderr, "evidence: ignored browser results of "+msg)
	}

	ev := evidence{
		Commit:      head,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Benchmarks:  map[string]string{},
		SizeBudgets: map[string]string{},
	}
	for _, row := range rows {
		names := map[string]bool{}
		for _, t := range row.Tests {
			names[t] = true
		}
		for _, t := range scan.byID[row.ID] {
			names[t] = true
		}
		sorted := make([]string, 0, len(names))
		for n := range names {
			sorted = append(sorted, n)
		}
		sort.Strings(sorted)
		ie := idEvidence{ID: row.ID, Status: row.Status}
		for _, n := range sorted {
			res := results[n]
			if res == "" {
				res = browserResult(n, browser)
			}
			if res == "" {
				res = "not_run"
			}
			ie.Tests = append(ie.Tests, testResult{Name: n, Result: res})
		}
		ev.IDs = append(ev.IDs, ie)
	}

	data, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := filepath.Join(dir, "evidence.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("evidence: wrote %s for %s (%d ids)\n", path, short(head), len(ev.IDs))
	return nil
}

func checkEvidence(root, dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, "evidence.json"))
	if err != nil {
		return fmt.Errorf("read evidence: %w", err)
	}
	var ev evidence
	if err := json.Unmarshal(data, &ev); err != nil {
		return fmt.Errorf("parse evidence: %w", err)
	}
	head, err := headCommit(root)
	if err != nil {
		return err
	}
	if ev.Commit != head {
		return fmt.Errorf("evidence is for %s but HEAD is %s; run just evidence", short(ev.Commit), short(head))
	}
	rows, err := readLedger(dir)
	if err != nil {
		return err
	}
	pass, err := checkRows(ev, rows)
	if err != nil {
		return err
	}
	fmt.Printf("evidence-check: %d PASS ids covered, evidence is for %s\n", pass, short(head))
	return nil
}

// checkRows returns the count of PASS ids. It fails when a PASS id has no
// passing covering test or has a failed one.
func checkRows(ev evidence, rows []ledgerRow) (int, error) {
	byID := map[string]idEvidence{}
	for _, ie := range ev.IDs {
		byID[ie.ID] = ie
	}
	ledgerIDs := map[string]bool{}
	pass := 0
	for _, row := range rows {
		ledgerIDs[row.ID] = true
		if row.Status != "PASS" {
			continue
		}
		pass++
		ie, ok := byID[row.ID]
		if !ok {
			return 0, fmt.Errorf("%s is PASS but missing from evidence", row.ID)
		}
		covered := false
		for _, t := range ie.Tests {
			switch t.Result {
			case "pass":
				covered = true
			case "fail":
				return 0, fmt.Errorf("%s is PASS but its covering test %q failed", row.ID, t.Name)
			}
		}
		if !covered {
			return 0, fmt.Errorf("%s is PASS with no passing covering test in evidence", row.ID)
		}
	}
	for id := range byID {
		if !ledgerIDs[id] {
			return 0, fmt.Errorf("evidence names %s, which is not in the ledger", id)
		}
	}
	return pass, nil
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func appendUnique(xs []string, x string) []string {
	for _, v := range xs {
		if v == x {
			return xs
		}
	}
	return append(xs, x)
}
