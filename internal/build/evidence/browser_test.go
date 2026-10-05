package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The report shape of bun test --reporter=junit (bun 1.2.13).
const junitReport = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="bun test" tests="5" assertions="3" failures="1" skipped="1" time="0.06">
  <testsuite name="a.spec.ts" tests="5" assertions="3" failures="1" skipped="1" time="0.002" hostname="host">
    <testcase name="REQ-REG-11 a toast lands" classname="" time="0" file="a.spec.ts" assertions="1" />
    <testcase name="REQ-REG-08 a&lt;b &amp; &quot;c&quot;: matches" classname="" time="0" file="a.spec.ts" assertions="1" />
    <testcase name="REQ-REG-09 no axe violations" classname="grp" time="0.002" file="a.spec.ts" assertions="1">
      <failure type="AssertionError" />
    </testcase>
    <testcase name="REQ-REG-12 left out" classname="" time="0" file="a.spec.ts" assertions="0">
      <skipped />
    </testcase>
    <testsuite name="nested">
      <testcase name="REQ-REG-13 nested" classname="" time="0" file="a.spec.ts" assertions="1" />
    </testsuite>
  </testsuite>
</testsuites>
`

func TestEvidence_JUnitResults(t *testing.T) {
	got, err := parseJUnit([]byte(junitReport))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"REQ-REG-11 a toast lands":      "pass",
		`REQ-REG-08 a<b & "c": matches`: "pass",
		"REQ-REG-09 no axe violations":  "fail",
		"REQ-REG-12 left out":           "skip",
		"REQ-REG-13 nested":             "pass",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d results, want %d: %v", len(got), len(want), got)
	}
	for name, res := range want {
		if got[name] != res {
			t.Errorf("%q: got %q, want %q", name, got[name], res)
		}
	}
}

func TestEvidence_StaleBrowserRunDoesNotCount(t *testing.T) {
	const head = "1111111111111111111111111111111111111111"
	dir := t.TempDir()
	report := func(file, title, stamp string) {
		t.Helper()
		xml := `<testsuites><testsuite><testcase name="` + title + `" /></testsuite></testsuites>`
		if err := os.WriteFile(filepath.Join(dir, file+".xml"), []byte(xml), 0o644); err != nil {
			t.Fatal(err)
		}
		if stamp == "" {
			return
		}
		if err := os.WriteFile(filepath.Join(dir, file+".json"), []byte(stamp), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report("head.spec.ts", "at head", `{"commit":"`+head+`","clean":true}`)
	report("dirty.spec.ts", "dirty tree", `{"commit":"`+head+`","clean":false}`)
	report("old.spec.ts", "other commit", `{"commit":"2222222222222222222222222222222222222222","clean":true}`)
	report("nogit.spec.ts", "no commit", `{"commit":"","clean":false}`)
	report("bare.spec.ts", "no stamp", "")

	got, ignored, err := readBrowserResults(dir, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["at head"] != "pass" {
		t.Errorf("only the clean run at HEAD counts, got %v", got)
	}
	if len(ignored) != 4 {
		t.Errorf("want 4 ignored reports, got %v", ignored)
	}

	got, _, err = readBrowserResults(filepath.Join(dir, "absent"), head)
	if err != nil || len(got) != 0 {
		t.Errorf("no results directory: got %v, %v", got, err)
	}
}

func TestEvidence_TemplateTitleNeedsEveryInstance(t *testing.T) {
	const title = "REQ-REG-08 ${entry.ref}: light, dark and focus match (1)"
	results := map[string]string{
		"REQ-REG-08 button: light, dark and focus match (1)": "pass",
		"REQ-REG-08 card: light, dark and focus match (1)":   "pass",
		"REQ-REG-08 card: light, dark and focus match (12)":  "fail",
		"REQ-REG-13 firefox: popover works":                  "pass",
	}
	if got := browserResult(title, results); got != "pass" {
		t.Errorf("all instances pass: got %q", got)
	}
	results["REQ-REG-08 card: light, dark and focus match (1)"] = "fail"
	if got := browserResult(title, results); got != "fail" {
		t.Errorf("one instance fails: got %q", got)
	}
	if got := browserResult("REQ-REG-07 ${name}: a menu flips", results); got != "" {
		t.Errorf("no instance ran: got %q", got)
	}
	// The source scan cuts a title at a quote inside an expression.
	if got := browserResult("REQ-REG-13 ${names[", results); got != "pass" {
		t.Errorf("cut title: got %q", got)
	}
	if got := browserResult("REQ-REG-13 firefox: popover works", results); got != "pass" {
		t.Errorf("plain title: got %q", got)
	}
}

func TestEvidence_CheckRows(t *testing.T) {
	rows := []ledgerRow{{ID: "REQ-REG-08", Status: "PASS"}, {ID: "REQ-REG-09", Status: "OPEN"}}
	ev := func(results ...string) evidence {
		ie := idEvidence{ID: "REQ-REG-08", Status: "PASS"}
		for _, r := range results {
			ie.Tests = append(ie.Tests, testResult{Name: "t-" + r, Result: r})
		}
		return evidence{IDs: []idEvidence{ie, {ID: "REQ-REG-09", Status: "OPEN"}}}
	}
	if n, err := checkRows(ev("not_run", "pass"), rows); err != nil || n != 1 {
		t.Errorf("a passing test covers the id: got %d, %v", n, err)
	}
	if _, err := checkRows(ev("not_run"), rows); err == nil || !strings.Contains(err.Error(), "no passing covering test") {
		t.Errorf("no result: got %v", err)
	}
	if _, err := checkRows(ev("pass", "fail"), rows); err == nil || !strings.Contains(err.Error(), `"t-fail" failed`) {
		t.Errorf("a failed test beside a passing one: got %v", err)
	}
}
