// Package widgetpkg is the package of the widgets of an app for a host: the
// check of the contract against the last published baseline (REQ-ISL-13),
// and the npm tarball and its publish (REQ-ISL-14).
package widgetpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// Level is the version bump that a change of the contract needs.
type Level int

const (
	// None is a contract with no change.
	None Level = iota
	// Minor is an addition.
	Minor
	// Major is a breaking change: a host that uses the old contract fails.
	Major
)

func (l Level) String() string {
	switch l {
	case Major:
		return "major"
	case Minor:
		return "minor"
	}
	return "none"
}

// Change is one difference between the baseline and the manifest of now.
type Change struct {
	Level Level
	Text  string
}

type typeText struct {
	Text string `json:"text"`
}

// manifest holds the parts of custom-elements.json that are the contract of
// a widget with its host.
type manifest struct {
	Modules []struct {
		Declarations []struct {
			TagName    string `json:"tagName"`
			Attributes []struct {
				Name string   `json:"name"`
				Type typeText `json:"type"`
			} `json:"attributes"`
			Events []struct {
				Name     string    `json:"name"`
				Type     typeText  `json:"type"`
				Contract *typeText `json:"contract"`
				Detail   []struct {
					Name     string   `json:"name"`
					Type     typeText `json:"type"`
					Optional bool     `json:"optional"`
				} `json:"detail"`
			} `json:"events"`
			CSSProperties []struct {
				Name string `json:"name"`
			} `json:"cssProperties"`
		} `json:"declarations"`
	} `json:"modules"`
}

// contract is the contract of one widget: each part by its name, with the
// text of its type.
type contract struct {
	attributes, events, fields, variables map[string]string
}

func contracts(data []byte) (map[string]contract, error) {
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	out := map[string]contract{}
	for _, mod := range m.Modules {
		for _, d := range mod.Declarations {
			if d.TagName == "" {
				continue
			}
			c := contract{attributes: map[string]string{}, events: map[string]string{}, fields: map[string]string{}, variables: map[string]string{}}
			for _, a := range d.Attributes {
				c.attributes[a.Name] = a.Type.Text
			}
			for _, e := range d.Events {
				c.events[e.Name] = e.Type.Text
				if e.Contract != nil {
					c.events[e.Name] = "CustomEvent<" + e.Contract.Text + ">"
				}
				for _, f := range e.Detail {
					text := f.Type.Text
					if f.Optional {
						text += ", optional"
					}
					c.fields[f.Name+" of the detail of "+e.Name] = text
				}
			}
			for _, v := range d.CSSProperties {
				c.variables[v.Name] = ""
			}
			out[d.TagName] = c
		}
	}
	return out, nil
}

// Compare returns the changes from the manifest of the baseline to the
// manifest of now (REQ-ISL-13). A removed or retyped attribute, event, event
// detail field or CSS variable is a major change, and so is a removed
// widget. An addition is a minor change.
func Compare(baseline, now []byte) ([]Change, error) {
	old, err := contracts(baseline)
	if err != nil {
		return nil, fmt.Errorf("the manifest of the baseline: %w", err)
	}
	cur, err := contracts(now)
	if err != nil {
		return nil, fmt.Errorf("the manifest: %w", err)
	}
	var out []Change
	for _, tag := range sortedKeys(old, cur) {
		o, inOld := old[tag]
		c, inCur := cur[tag]
		switch {
		case !inCur:
			out = append(out, Change{Major, "the widget " + tag + " is removed"})
			continue
		case !inOld:
			out = append(out, Change{Minor, "the widget " + tag + " is new"})
			continue
		}
		for _, part := range []struct {
			kind     string
			old, cur map[string]string
		}{
			{"attribute", o.attributes, c.attributes},
			{"event", o.events, c.events},
			{"field", o.fields, c.fields},
			{"CSS variable", o.variables, c.variables},
		} {
			for _, name := range sortedKeys(part.old, part.cur) {
				before, inOld := part.old[name]
				after, inCur := part.cur[name]
				what := tag + ": the " + part.kind + " " + name
				switch {
				case !inCur:
					out = append(out, Change{Major, what + " is removed"})
				case !inOld:
					out = append(out, Change{Minor, what + " is new"})
				case before != after:
					out = append(out, Change{Major, what + " has the type " + after + "; the baseline has " + before})
				}
			}
		}
	}
	return out, nil
}

func sortedKeys[V any](maps ...map[string]V) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range maps {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Needed returns the highest level of the changes.
func Needed(changes []Change) Level {
	level := None
	for _, c := range changes {
		if c.Level > level {
			level = c.Level
		}
	}
	return level
}

// Version is a semver version with no pre-release part.
type Version struct{ Major, Minor, Patch int }

var versionPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// ParseVersion reads a version such as 1.4.0.
func ParseVersion(s string) (Version, error) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("the version %q is not three numbers such as 1.4.0", s)
	}
	var v Version
	for i, part := range []*int{&v.Major, &v.Minor, &v.Patch} {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return Version{}, fmt.Errorf("the version %q has a number that is too large", s)
		}
		*part = n
	}
	return v, nil
}

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

func (v Version) less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

// CheckBump returns an error when the version of now does not have the bump
// that the level needs from the version of the baseline.
func CheckBump(baseline, now Version, level Level) error {
	if now.less(baseline) {
		return fmt.Errorf("the version %s is lower than the version %s of the baseline", now, baseline)
	}
	switch level {
	case Major:
		if now.Major == baseline.Major {
			return fmt.Errorf("a breaking change needs a major bump: the baseline is %s, so the version must be %d.0.0 or higher; it is %s", baseline, baseline.Major+1, now)
		}
	case Minor:
		if now.Major == baseline.Major && now.Minor == baseline.Minor {
			return fmt.Errorf("an addition needs a minor bump: the baseline is %s, so the version must be %d.%d.0 or higher; it is %s", baseline, baseline.Major, baseline.Minor+1, now)
		}
	}
	return nil
}

// Baseline is the contract of the last published version. The app commits
// the file.
type Baseline struct {
	Name     string          `json:"name"`
	Version  string          `json:"version"`
	Manifest json.RawMessage `json:"manifest"`
}

// BaselinePath returns the file of the baseline under the app root. The
// scaffold keeps .gx/base/ in git.
func BaselinePath(root string) string { return filepath.Join(root, ".gx", "base", "widgets.json") }

// ReadBaseline reads the baseline of the app. An app with no baseline gives
// nil.
func ReadBaseline(root string) (*Baseline, error) {
	data, err := os.ReadFile(BaselinePath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("%s is not a baseline of the widgets: %w", BaselinePath(root), err)
	}
	return &b, nil
}

// WriteBaseline records the manifest of a version as the baseline.
func WriteBaseline(root string, b Baseline) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	path := BaselinePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
