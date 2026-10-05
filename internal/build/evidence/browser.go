package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// browserResultsDir is where tests/e2e/record.sh leaves, per test file, the
// JUnit report of the run and a stamp of the tree the run saw.
var browserResultsDir = filepath.Join("tests", "e2e", ".results")

type runStamp struct {
	Commit string `json:"commit"`
	Clean  bool   `json:"clean"`
}

type junitSuite struct {
	Suites []junitSuite `xml:"testsuite"`
	Cases  []junitCase  `xml:"testcase"`
}

type junitCase struct {
	Name    string    `xml:"name,attr"`
	Failure *struct{} `xml:"failure"`
	Error   *struct{} `xml:"error"`
	Skipped *struct{} `xml:"skipped"`
}

// readBrowserResults returns the result of each browser test title recorded
// in dir. A report counts only when its stamp says the run saw a clean tree
// at head; every other report is named in ignored.
func readBrowserResults(dir, head string) (results map[string]string, ignored []string, err error) {
	results = map[string]string{}
	reports, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	if err != nil {
		return nil, nil, err
	}
	for _, report := range reports {
		name := filepath.Base(report)
		var stamp runStamp
		data, err := os.ReadFile(strings.TrimSuffix(report, ".xml") + ".json")
		if err != nil {
			ignored = append(ignored, name+": no stamp")
			continue
		}
		if err := json.Unmarshal(data, &stamp); err != nil {
			return nil, nil, fmt.Errorf("parse stamp of %s: %w", name, err)
		}
		if stamp.Commit != head {
			ignored = append(ignored, fmt.Sprintf("%s: run at %s, not at HEAD", name, short(stamp.Commit)))
			continue
		}
		if !stamp.Clean {
			ignored = append(ignored, name+": run on a dirty tree")
			continue
		}
		data, err = os.ReadFile(report)
		if err != nil {
			return nil, nil, err
		}
		cases, err := parseJUnit(data)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", name, err)
		}
		for title, res := range cases {
			results[title] = worse(results[title], res)
		}
	}
	return results, ignored, nil
}

func parseJUnit(data []byte) (map[string]string, error) {
	var root junitSuite
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	cases := map[string]string{}
	var walk func(s junitSuite)
	walk = func(s junitSuite) {
		for _, c := range s.Cases {
			res := "pass"
			switch {
			case c.Failure != nil || c.Error != nil:
				res = "fail"
			case c.Skipped != nil:
				res = "skip"
			}
			cases[c.Name] = worse(cases[c.Name], res)
		}
		for _, sub := range s.Suites {
			walk(sub)
		}
	}
	walk(root)
	return cases, nil
}

// worse keeps the result that says least for a title that ran more than
// once: a fail beats a skip, a skip beats a pass.
func worse(a, b string) string {
	rank := map[string]int{"": 0, "pass": 1, "skip": 2, "fail": 3}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

// browserResult is the recorded result of a test title as the source spells
// it. A template literal title stands for every test it expands to: it
// passes only when at least one ran and all of them passed.
func browserResult(title string, results map[string]string) string {
	if !strings.Contains(title, "${") {
		return results[title]
	}
	re := titlePattern(title)
	res := ""
	for name, r := range results {
		if re.MatchString(name) {
			res = worse(res, r)
		}
	}
	return res
}

func titlePattern(title string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for {
		i := strings.Index(title, "${")
		if i < 0 {
			b.WriteString(regexp.QuoteMeta(title))
			break
		}
		b.WriteString(regexp.QuoteMeta(title[:i]))
		b.WriteString(".*")
		// The source scan stops a title at a quote, so an expression can
		// be cut short; the rest of the title is then unknown.
		j := strings.Index(title[i:], "}")
		if j < 0 {
			break
		}
		title = title[i+j+1:]
	}
	b.WriteString("$")
	return regexp.MustCompile("(?s)" + b.String())
}
