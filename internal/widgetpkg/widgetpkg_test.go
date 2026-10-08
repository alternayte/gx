package widgetpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// widgetJSON is the declaration of one widget in a manifest.
type widgetJSON struct {
	tag        string
	attributes map[string]string
	// events maps the name of an event to the fields of its detail; the
	// value of a field is its type, with a final ? for an optional field.
	events    map[string]map[string]string
	variables []string
}

func manifestOf(widgets ...widgetJSON) []byte {
	type typ struct {
		Text string `json:"text"`
	}
	type named struct {
		Name     string `json:"name"`
		Type     *typ   `json:"type,omitempty"`
		Optional bool   `json:"optional,omitempty"`
		Detail   []any  `json:"detail,omitempty"`
	}
	var modules []any
	for _, w := range widgets {
		var attrs, events, vars []any
		for name, t := range w.attributes {
			attrs = append(attrs, named{Name: name, Type: &typ{t}})
		}
		for name, fields := range w.events {
			e := named{Name: name, Type: &typ{"CustomEvent<Detail>"}}
			for field, t := range fields {
				text, optional := strings.CutSuffix(t, "?")
				e.Detail = append(e.Detail, named{Name: field, Type: &typ{text}, Optional: optional})
			}
			events = append(events, e)
		}
		for _, v := range w.variables {
			vars = append(vars, named{Name: v})
		}
		modules = append(modules, map[string]any{"declarations": []any{map[string]any{
			"tagName": w.tag, "attributes": attrs, "events": events, "cssProperties": vars,
		}}})
	}
	data, _ := json.Marshal(map[string]any{"schemaVersion": "1.0.0", "modules": modules})
	return data
}

func cart() widgetJSON {
	return widgetJSON{
		tag:        "acme-cart",
		attributes: map[string]string{"currency": "string", "compact": "boolean"},
		events:     map[string]map[string]string{"cart-changed": {"count": "number", "note": "string?"}},
		variables:  []string{"--primary", "--radius"},
	}
}

// TestREQ_ISL_13_Classification checks the class of each change of the
// contract of a widget: a removed or retyped attribute, event, event detail
// field or CSS variable is breaking, and an addition is minor.
func TestREQ_ISL_13_Classification(t *testing.T) {
	cases := []struct {
		name   string
		change func(*widgetJSON)
		level  Level
		text   string
	}{
		{"no change", func(w *widgetJSON) {}, None, ""},
		{"removed attribute", func(w *widgetJSON) { delete(w.attributes, "compact") }, Major, "acme-cart: the attribute compact is removed"},
		{"retyped attribute", func(w *widgetJSON) { w.attributes["compact"] = "number" }, Major, "acme-cart: the attribute compact has the type number; the baseline has boolean"},
		{"new attribute", func(w *widgetJSON) { w.attributes["locale"] = "string" }, Minor, "acme-cart: the attribute locale is new"},
		{"removed event", func(w *widgetJSON) { delete(w.events, "cart-changed") }, Major, "acme-cart: the event cart-changed is removed"},
		{"new event", func(w *widgetJSON) { w.events["cart-emptied"] = nil }, Minor, "acme-cart: the event cart-emptied is new"},
		{"removed detail field", func(w *widgetJSON) { delete(w.events["cart-changed"], "count") }, Major, "acme-cart: the field count of the detail of cart-changed is removed"},
		{"retyped detail field", func(w *widgetJSON) { w.events["cart-changed"]["count"] = "string" }, Major, "acme-cart: the field count of the detail of cart-changed has the type string; the baseline has number"},
		{"detail field that is optional now", func(w *widgetJSON) { w.events["cart-changed"]["count"] = "number?" }, Major, "acme-cart: the field count of the detail of cart-changed has the type number, optional; the baseline has number"},
		{"new detail field", func(w *widgetJSON) { w.events["cart-changed"]["total"] = "number" }, Minor, "acme-cart: the field total of the detail of cart-changed is new"},
		{"removed CSS variable", func(w *widgetJSON) { w.variables = []string{"--primary"} }, Major, "acme-cart: the CSS variable --radius is removed"},
		{"new CSS variable", func(w *widgetJSON) { w.variables = append(w.variables, "--ring") }, Minor, "acme-cart: the CSS variable --ring is new"},
		{"an addition and a removal", func(w *widgetJSON) { w.attributes["locale"] = "string"; delete(w.attributes, "compact") }, Major, "acme-cart: the attribute compact is removed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := cart()
			tc.change(&now)
			changes, err := Compare(manifestOf(cart()), manifestOf(now))
			if err != nil {
				t.Fatal(err)
			}
			if got := Needed(changes); got != tc.level {
				t.Fatalf("level = %v, want %v; changes %v", got, tc.level, changes)
			}
			if tc.text == "" {
				if len(changes) != 0 {
					t.Fatalf("changes = %v", changes)
				}
				return
			}
			for _, c := range changes {
				if c.Text == tc.text && c.Level == tc.level {
					return
				}
			}
			t.Fatalf("no change %q in %v", tc.text, changes)
		})
	}
	t.Run("a removed widget and a new widget", func(t *testing.T) {
		composer := widgetJSON{tag: "acme-composer"}
		changes, err := Compare(manifestOf(cart()), manifestOf(composer))
		if err != nil {
			t.Fatal(err)
		}
		want := []Change{{Major, "the widget acme-cart is removed"}, {Minor, "the widget acme-composer is new"}}
		if len(changes) != 2 || changes[0] != want[0] || changes[1] != want[1] {
			t.Fatalf("changes = %v", changes)
		}
	})
}

// TestREQ_ISL_13_Bump checks the version that each class of change needs.
func TestREQ_ISL_13_Bump(t *testing.T) {
	cases := []struct {
		baseline, now string
		level         Level
		ok            bool
	}{
		{"1.2.3", "1.2.3", None, true},
		{"1.2.3", "1.2.4", None, true},
		{"1.2.3", "1.2.2", None, false},
		{"1.2.3", "1.2.4", Minor, false},
		{"1.2.3", "1.3.0", Minor, true},
		{"1.2.3", "2.0.0", Minor, true},
		{"1.2.3", "1.3.0", Major, false},
		{"1.2.3", "2.0.0", Major, true},
		{"0.4.0", "0.5.0", Major, false},
		{"0.4.0", "1.0.0", Major, true},
		{"2.0.0", "1.9.9", Major, false},
	}
	for _, tc := range cases {
		base, err := ParseVersion(tc.baseline)
		if err != nil {
			t.Fatal(err)
		}
		now, err := ParseVersion(tc.now)
		if err != nil {
			t.Fatal(err)
		}
		if err := CheckBump(base, now, tc.level); (err == nil) != tc.ok {
			t.Errorf("%s to %s for a %v change: %v, want ok = %v", tc.baseline, tc.now, tc.level, err, tc.ok)
		}
	}
	for _, bad := range []string{"", "1.2", "v1.2.3", "1.2.3-beta", "01.2.3", "1.2.x"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) gave no error", bad)
		}
	}
}

func cartPackage() Package {
	return Package{
		Name: "@acme/widgets", Version: "1.2.0", Tags: []string{"acme-cart"},
		Files: []File{
			{"acme-cart.js", []byte("customElements.define('acme-cart', class extends HTMLElement {})\n")},
			{"acme-cart.d.ts", []byte("export declare class AcmeCartElement extends HTMLElement {}\n")},
			{"acme-cart.react.d.ts", []byte("export {}\n")},
			{ManifestFile, manifestOf(cart())},
		},
	}
}

// untar returns the files of an npm tarball by name.
func untar(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for tr := tar.NewReader(zr); ; {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		if !h.ModTime.IsZero() && h.ModTime.Unix() != 0 {
			t.Errorf("%s has the time %v", h.Name, h.ModTime)
		}
		out[h.Name] = string(body)
	}
}

// TestREQ_ISL_14_Tarball checks the npm tarball: the element file, the type
// files and the manifest below package/, a package.json that names them,
// and the same bytes for the same files.
func TestREQ_ISL_14_Tarball(t *testing.T) {
	p := cartPackage()
	data, err := p.Tarball()
	if err != nil {
		t.Fatal(err)
	}
	files := untar(t, data)
	for _, name := range []string{"package/package.json", "package/acme-cart.js", "package/acme-cart.d.ts", "package/acme-cart.react.d.ts", "package/custom-elements.json"} {
		if _, ok := files[name]; !ok {
			t.Errorf("the tarball has no %s; it has %d files", name, len(files))
		}
	}
	if len(files) != 5 {
		t.Errorf("the tarball has %d files", len(files))
	}
	var meta struct {
		Name, Version, Type, CustomElements string
		Files                               []string
		Exports                             map[string]map[string]string
	}
	if err := json.Unmarshal([]byte(files["package/package.json"]), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Name != "@acme/widgets" || meta.Version != "1.2.0" || meta.Type != "module" || meta.CustomElements != "custom-elements.json" {
		t.Errorf("package.json = %+v", meta)
	}
	if e := meta.Exports["./acme-cart"]; e["types"] != "./acme-cart.d.ts" || e["default"] != "./acme-cart.js" {
		t.Errorf("exports = %v", meta.Exports)
	}
	if e := meta.Exports["."]; e["default"] != "./acme-cart.js" {
		t.Errorf("a package of one widget has no main export: %v", meta.Exports)
	}
	// A resolver takes the first condition that matches: types is first.
	if text := files["package/package.json"]; strings.Index(text, `"types": "./acme-cart.d.ts"`) > strings.Index(text, `"default": "./acme-cart.js"`) {
		t.Errorf("the default condition is before the types condition:\n%s", text)
	}
	again, err := p.Tarball()
	if err != nil || !bytes.Equal(data, again) {
		t.Errorf("a second tarball of the same files has different bytes (%v)", err)
	}
	if got := TarballName("@acme/widgets", "1.2.0"); got != "acme-widgets-1.2.0.tgz" {
		t.Errorf("TarballName = %s", got)
	}
	two := cartPackage()
	two.Tags = append(two.Tags, "acme-composer")
	if text, err := two.PackageJSON(); err != nil || strings.Contains(string(text), `".":`) {
		t.Errorf("a package of two widgets has a main export (%v)", err)
	}
	for _, bad := range []string{"", "Acme", "@acme", "acme widgets", "@acme/a/b"} {
		p := cartPackage()
		p.Name = bad
		if _, err := p.Tarball(); err == nil {
			t.Errorf("the name %q gave no error", bad)
		}
	}
}

// TestREQ_ISL_14_Publish publishes to a registry fixture through the npm
// registry HTTP API: one PUT with the token, the version document and the
// tarball.
func TestREQ_ISL_14_Publish(t *testing.T) {
	type attachment struct {
		ContentType string `json:"content_type"`
		Data        string `json:"data"`
		Length      int    `json:"length"`
	}
	var got struct {
		path, auth string
		doc        struct {
			ID       string            `json:"_id"`
			Name     string            `json:"name"`
			Access   string            `json:"access"`
			DistTags map[string]string `json:"dist-tags"`
			Versions map[string]struct {
				ID   string `json:"_id"`
				Name string `json:"name"`
				Dist struct {
					Integrity, Shasum, Tarball string
				} `json:"dist"`
			} `json:"versions"`
			Attachments map[string]attachment `json:"_attachments"`
		}
	}
	published := map[string]bool{}
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"the token is not valid"}`))
			return
		}
		got.path, got.auth = r.URL.EscapedPath(), r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got.doc); err != nil {
			http.Error(w, `{"error":"body"}`, http.StatusBadRequest)
			return
		}
		for v := range got.doc.Versions {
			if published[v] {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"You cannot publish over the previously published versions: ` + v + `."}`))
				return
			}
			published[v] = true
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer registry.Close()

	p := cartPackage()
	opt := PublishOptions{Registry: registry.URL + "/", Token: "secret-token", Access: "public"}
	if err := Publish(context.Background(), p, opt); err != nil {
		t.Fatal(err)
	}
	if got.path != "/@acme%2fwidgets" {
		t.Errorf("path = %s", got.path)
	}
	doc := got.doc
	if doc.ID != "@acme/widgets" || doc.Name != "@acme/widgets" || doc.Access != "public" || doc.DistTags["latest"] != "1.2.0" {
		t.Errorf("document = %+v", doc)
	}
	version := doc.Versions["1.2.0"]
	file, ok := doc.Attachments["acme-widgets-1.2.0.tgz"]
	if !ok || file.ContentType != "application/octet-stream" {
		t.Fatalf("attachments = %v", doc.Attachments)
	}
	tarball, err := base64.StdEncoding.DecodeString(file.Data)
	if err != nil || len(tarball) != file.Length {
		t.Fatalf("the attachment: %v, %d bytes, length %d", err, len(tarball), file.Length)
	}
	sum1, sum512 := sha1.Sum(tarball), sha512.Sum512(tarball)
	if version.ID != "@acme/widgets@1.2.0" || version.Name != "@acme/widgets" ||
		version.Dist.Shasum != hex.EncodeToString(sum1[:]) ||
		version.Dist.Integrity != "sha512-"+base64.StdEncoding.EncodeToString(sum512[:]) ||
		version.Dist.Tarball != registry.URL+"/@acme/widgets/-/acme-widgets-1.2.0.tgz" {
		t.Errorf("version = %+v", version)
	}
	if files := untar(t, tarball); !strings.Contains(files["package/acme-cart.js"], "customElements.define") {
		t.Errorf("the tarball of the registry has no element file")
	}

	// The registry refuses a version that it has, and says why.
	if err := Publish(context.Background(), p, opt); err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "previously published") {
		t.Errorf("a second publish of one version: %v", err)
	}
	// A token that the registry refuses, and no token.
	opt.Token = "wrong"
	if err := Publish(context.Background(), p, opt); err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "wrong") {
		t.Errorf("a wrong token: %v", err)
	}
	opt.Token = ""
	if err := Publish(context.Background(), p, opt); err == nil {
		t.Error("no token gave no error")
	}
}
