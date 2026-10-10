// Package fuzz is `gx fuzz` (REQ-AI-11): it runs the dev build of an app,
// asks it to render random prop sets of each component, and checks each
// render: no panic, the HTML parses to the tree that the render wrote, and
// axe finds no violation in headless Chrome. No node.
package fuzz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/alternayte/gx/internal/appmodel"
	"github.com/alternayte/gx/internal/apprun"
	"github.com/alternayte/gx/internal/chrome"
	"github.com/alternayte/gx/internal/propgen"
)

// Options select the run.
type Options struct {
	// Dir is the app module root.
	Dir string
	// Main is the app main package; apprun.DetectMain finds it when empty.
	Main string
	// Component limits the run to the components of this name. It can
	// have the package name before a dot: "cart.Cart".
	Component string
	// Seed makes the prop sets. One seed gives the same sets each run.
	Seed uint64
	// Sets is the number of prop sets of each component.
	Sets int
}

// Failure is one prop set that fails a check.
type Failure struct {
	Component string `json:"component"`
	Package   string `json:"package"`
	Index     int    `json:"index"`
	// Kind is "panic", "tree" or "a11y".
	Kind    string `json:"kind"`
	Message string `json:"message"`
	// Name and Fixture are the entry for the fixtures file of the
	// component: `"<Name>": <Fixture>,`. Fixture is empty when Go source
	// cannot hold the prop set; Unprintable then says why.
	Name        string   `json:"name"`
	Fixture     string   `json:"fixture"`
	Imports     []string `json:"imports,omitempty"`
	Unprintable string   `json:"unprintable,omitempty"`
}

// Result is the outcome of one run.
type Result struct {
	Seed       uint64 `json:"seed"`
	Components int    `json:"components"`
	// Sets is the number of prop sets that the run rendered.
	Sets     int             `json:"sets"`
	Skipped  []string        `json:"skipped"`
	Failures []Failure       `json:"failures"`
	Report   *propgen.Report `json:"-"`
}

// Run does one run. The error is for a run that cannot start or finish:
// the app does not build, or Chrome does not start. A failing prop set is
// in the result.
func Run(ctx context.Context, opt Options) (*Result, error) {
	app, err := apprun.Start(ctx, opt.Dir, opt.Main)
	if err != nil {
		return nil, fmt.Errorf("the app does not build: %w", err)
	}
	defer app.Stop()

	component, pkgName := opt.Component, ""
	if i := strings.LastIndex(component, "."); i >= 0 {
		pkgName, component = component[:i], component[i+1:]
	}
	q := url.Values{"seed": {strconv.FormatUint(opt.Seed, 10)}, "n": {strconv.Itoa(opt.Sets)}}
	if component != "" {
		q.Set("component", component)
	}
	report, err := fetch(ctx, app.Base+"/_gx/fuzz?"+q.Encode())
	if err != nil {
		return nil, err
	}

	required, err := requiredProps(opt.Dir)
	if err != nil {
		return nil, err
	}

	res := &Result{Seed: opt.Seed, Skipped: []string{}, Failures: []Failure{}, Report: report}
	var browser chrome.Browser
	defer browser.Close()
	for _, t := range report.Targets {
		if pkgName != "" && path.Base(t.Package) != pkgName && t.Package != pkgName {
			continue
		}
		if t.Skipped != "" {
			res.Skipped = append(res.Skipped, t.Package+"."+t.Component+": "+t.Skipped)
			continue
		}
		if prop := firstRequired(required[t.Package+"\x00"+t.Component], t.CannotMake); prop != "" {
			res.Skipped = append(res.Skipped, t.Package+"."+t.Component+": gx fuzz cannot make a value for the required prop "+prop)
			continue
		}
		res.Components++
		for _, c := range t.Cases {
			res.Sets++
			f := Failure{
				Component: t.Component, Package: t.Package, Index: c.Index,
				Name:    fmt.Sprintf("Fuzz%dSet%d", opt.Seed, c.Index),
				Fixture: c.Fixture, Imports: c.Imports, Unprintable: c.Unprintable,
			}
			if c.Panic != "" {
				f.Kind, f.Message = "panic", "the render panics: "+c.Panic
				res.Failures = append(res.Failures, f)
				continue
			}
			// Text in a slot that takes only rows or list items is a
			// defect of the caller: the tree with empty slots decides.
			if d := TreeDefect(c.HTML); d != "" && (!c.Slots || TreeDefect(c.Bare) != "") {
				f.Kind, f.Message = "tree", "the HTML does not parse to the tree that the render wrote: "+d
				res.Failures = append(res.Failures, f)
				continue
			}
			rq := url.Values{"seed": q["seed"], "n": {strconv.Itoa(c.Index)}, "component": {t.Component}, "package": {t.Package}}
			// The audit is for the component, not for the page around it.
			audit, err := browser.Audit(ctx, app.Base+"/_gx/fuzz/render?"+rq.Encode(), "#gx-fixture")
			if err != nil {
				return nil, err
			}
			if len(audit.Violations) > 0 {
				var parts []string
				for _, v := range audit.Violations {
					parts = append(parts, v.ID+" ("+v.Help+")")
				}
				f.Kind, f.Message = "a11y", "axe reports "+strings.Join(parts, "; ")
				res.Failures = append(res.Failures, f)
			}
		}
	}
	if component != "" && res.Components == 0 && len(res.Skipped) == 0 {
		return nil, fmt.Errorf("the app has no component %s", opt.Component)
	}
	return res, nil
}

// requiredProps returns the names of the required props of each component,
// by the import path and the name of the component.
func requiredProps(dir string) (map[string]map[string]bool, error) {
	model, diags, err := appmodel.Describe(dir)
	if err != nil {
		return nil, err
	}
	if len(diags) > 0 {
		return nil, fmt.Errorf("%s", diags[0].String())
	}
	out := map[string]map[string]bool{}
	for _, c := range model.Components {
		names := map[string]bool{}
		for _, p := range c.Props {
			if p.Required {
				names[p.Name] = true
			}
		}
		out[c.Package+"\x00"+c.Name] = names
	}
	return out, nil
}

// firstRequired returns the first name of the list that is required.
func firstRequired(required map[string]bool, names []string) string {
	for _, name := range names {
		if required[name] {
			return name
		}
	}
	return ""
}

func fetch(ctx context.Context, target string) (*propgen.Report, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("the app has no fuzz route; its gx module is older than this gx command")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the app answers %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var report propgen.Report
	if err := json.Unmarshal(body, &report); err != nil {
		return nil, fmt.Errorf("the fuzz report of the app: %w", err)
	}
	return &report, nil
}

// Print writes the result for a person: each failing prop set as a
// fixture, then one summary line with the seed.
func (r *Result) Print(w io.Writer) {
	for _, f := range r.Failures {
		fmt.Fprintf(w, "%s.%s, set %d: %s\n", f.Package, f.Component, f.Index, f.Message)
		if f.Fixture == "" {
			fmt.Fprintf(w, "  no fixture: %s\n", f.Unprintable)
			continue
		}
		fmt.Fprintf(w, "  fixture for %s.fixtures.go:\n", f.Component)
		fmt.Fprintf(w, "    %q: %s,\n", f.Name, f.Fixture)
		if len(f.Imports) > 0 {
			fmt.Fprintf(w, "  imports: %s\n", strings.Join(f.Imports, ", "))
		}
	}
	for _, s := range r.Skipped {
		fmt.Fprintf(w, "skipped %s\n", s)
	}
	fmt.Fprintf(w, "gx fuzz: %d components, %d prop sets, %d failures, seed %d\n", r.Components, r.Sets, len(r.Failures), r.Seed)
}
