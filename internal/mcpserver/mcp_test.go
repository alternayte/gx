package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alternayte/gx/internal/mcpserver"
	"github.com/alternayte/gx/internal/registry"
	"github.com/alternayte/gx/internal/scaffold"
)

// The contract tests share one scaffolded app, one server and one client
// session: the server builds the app once and starts one Chrome.
var shared struct {
	once    sync.Once
	err     error
	dir     string
	session *mcp.ClientSession
	server  *mcpserver.Server
	cancel  context.CancelFunc
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func setup() error {
	parent, err := os.MkdirTemp("", "gx-mcp-test-")
	if err != nil {
		return err
	}
	dir := filepath.Join(parent, "acme")
	if _, err := scaffold.Init(scaffold.Options{
		Dir: dir, Module: "example.com/acme", Adapter: "datastar", Version: "0.1.0", Replace: repoRoot(),
	}); err != nil {
		return err
	}
	// A component with a known accessibility fault: an image with no text
	// alternative.
	if _, err := scaffold.New(dir, "component", "ui/bad/Bad"); err != nil {
		return err
	}
	bad := "package bad\n\nprops {\n  // Children is the content.\n  Children gx.Node\n  // Attrs are the attributes of the image.\n  Attrs gx.Attrs = nil\n}\n\n<div><img src=\"data:image/gif;base64,R0lGODlhAQABAAAAACw=\" width=\"20\" height=\"20\" {...p.Attrs} />{p.Children}</div>\n"
	if err := os.WriteFile(filepath.Join(dir, "ui", "bad", "Bad.gx"), []byte(bad), 0o644); err != nil {
		return err
	}
	if _, err := scaffold.Generate(dir); err != nil {
		return err
	}
	shared.dir = dir
	shared.server = mcpserver.New(mcpserver.Options{
		Dir:     dir,
		Version: "0.1.0",
		Installer: func() (registry.Installer, error) {
			return registry.Installer{Root: dir, Source: filepath.Join(repoRoot(), "registry"), Dir: "ui"}, nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	shared.cancel = cancel
	serverSide, clientSide := mcp.NewInMemoryTransports()
	go func() { _ = shared.server.MCP().Run(ctx, serverSide) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "contract-test", Version: "0.0.0"}, nil)
	shared.session, err = client.Connect(ctx, clientSide, nil)
	return err
}

func TestMain(m *testing.M) {
	// The apps of the tests resolve the gx module through a replace.
	os.Setenv("GOFLAGS", "-mod=mod")
	code := m.Run()
	if shared.session != nil {
		_ = shared.session.Close()
	}
	if shared.cancel != nil {
		shared.cancel()
	}
	if shared.server != nil {
		shared.server.Close()
	}
	if shared.dir != "" {
		_ = os.RemoveAll(filepath.Dir(shared.dir))
	}
	os.Exit(code)
}

func session(t *testing.T) *mcp.ClientSession {
	t.Helper()
	shared.once.Do(func() { shared.err = setup() })
	if shared.err != nil {
		t.Fatalf("setup: %v", shared.err)
	}
	return shared.session
}

// call runs one tool and decodes its structured result into out.
func call(t *testing.T, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	res, err := session(t).CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s answered an error: %s", name, textOf(res))
	}
	if out != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("%s: structured result: %v\n%s", name, err, raw)
		}
	}
	return res
}

func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

// pngOf returns the image of a result and checks that it is a real PNG.
func pngOf(t *testing.T, res *mcp.CallToolResult) (width, height int) {
	t.Helper()
	for _, c := range res.Content {
		img, ok := c.(*mcp.ImageContent)
		if !ok {
			continue
		}
		if img.MIMEType != "image/png" {
			t.Fatalf("image type = %q", img.MIMEType)
		}
		decoded, err := png.Decode(bytes.NewReader(img.Data))
		if err != nil {
			t.Fatalf("the image is not a PNG: %v", err)
		}
		return decoded.Bounds().Dx(), decoded.Bounds().Dy()
	}
	t.Fatalf("the result holds no image: %s", textOf(res))
	return 0, 0
}

// TestREQ_AI_04_ToolList covers the tool contract of `gx mcp`: the eight
// tools, each with a description and an object input schema (REQ-AI-04).
func TestREQ_AI_04_ToolList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	list, err := session(t).ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
		if len(tool.Description) < 20 {
			t.Errorf("%s has no useful description: %q", tool.Name, tool.Description)
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil || schema.Type != "object" {
			t.Errorf("%s input schema = %s", tool.Name, raw)
		}
	}
	sort.Strings(names)
	want := "a11y_audit check describe registry_add registry_search render_fixture routes screenshot_route"
	if got := strings.Join(names, " "); got != want {
		t.Fatalf("tools = %s\nwant    %s", got, want)
	}
}

// TestREQ_AI_04_DescribeCheckRoutes covers the three model tools: describe
// answers the app model, routes the route list, and check the diagnostics
// of a broken file (REQ-AI-04).
func TestREQ_AI_04_DescribeCheckRoutes(t *testing.T) {
	var described struct {
		Model struct {
			Module     string
			Components []struct{ Name, Package string }
			Actions    []struct{ Handler string }
		}
	}
	call(t, "describe", nil, &described)
	if described.Model.Module != "example.com/acme" || len(described.Model.Components) < 3 || len(described.Model.Actions) != 1 {
		t.Fatalf("describe = %+v", described)
	}

	var routes struct {
		Routes []struct{ Pattern, Page string }
	}
	call(t, "routes", nil, &routes)
	var home bool
	for _, r := range routes.Routes {
		if r.Pattern == "GET /{$}" && r.Page == "home.Page" {
			home = true
		}
	}
	if !home {
		t.Fatalf("routes = %+v", routes)
	}

	var checked struct {
		OK          bool
		Diagnostics []struct {
			Code, File, Message, Doc string
			Line                     int
		}
	}
	call(t, "check", nil, &checked)
	if !checked.OK || len(checked.Diagnostics) != 0 {
		t.Fatalf("check on a good app = %+v", checked)
	}
	broken := filepath.Join(shared.dir, "home", "Broken.gx")
	if err := os.WriteFile(broken, []byte("package home\n\n<Counter />\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(broken)
	call(t, "check", nil, &checked)
	if checked.OK || len(checked.Diagnostics) == 0 {
		t.Fatalf("check on a broken app = %+v", checked)
	}
	d := checked.Diagnostics[0]
	if d.Code != "GX2001" || !strings.HasSuffix(d.File, "Broken.gx") || d.Line != 3 || d.Doc != "/errors/GX2001" {
		t.Fatalf("diagnostic = %+v", d)
	}
}

// TestREQ_AI_04_RenderFixture covers render_fixture: the HTML of one
// fixture, its PNG from headless Chrome, and a clear error for a fixture
// that does not exist (REQ-AI-04).
func TestREQ_AI_04_RenderFixture(t *testing.T) {
	var out struct {
		HTML string
	}
	call(t, "render_fixture", map[string]any{"component": "Counter", "name": "Default"}, &out)
	for _, want := range []string{"Clicks", "data-signals", "Add one"} {
		if !strings.Contains(out.HTML, want) {
			t.Fatalf("fixture HTML lacks %q:\n%s", want, out.HTML)
		}
	}
	if strings.Contains(out.HTML, "<html") {
		t.Fatalf("fixture HTML is a whole document:\n%s", out.HTML)
	}

	res := call(t, "render_fixture", map[string]any{"component": "Counter", "name": "Default", "format": "png"}, nil)
	if w, h := pngOf(t, res); w < 200 || h < 50 {
		t.Fatalf("fixture PNG is %dx%d", w, h)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	missing, err := session(t).CallTool(ctx, &mcp.CallToolParams{Name: "render_fixture", Arguments: map[string]any{"component": "Counter", "name": "Nope"}})
	if err != nil {
		t.Fatal(err)
	}
	if !missing.IsError || !strings.Contains(textOf(missing), "Counter") || !strings.Contains(textOf(missing), "Default") {
		t.Fatalf("a missing fixture answered: error=%v %s", missing.IsError, textOf(missing))
	}
}

// TestREQ_AI_04_ScreenshotRoute covers screenshot_route: a PNG of a page
// at the asked viewport width (REQ-AI-04).
func TestREQ_AI_04_ScreenshotRoute(t *testing.T) {
	res := call(t, "screenshot_route", map[string]any{"path": "/", "width": 800, "height": 600}, nil)
	if w, h := pngOf(t, res); w != 800 || h < 300 {
		t.Fatalf("screenshot is %dx%d, want width 800", w, h)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	missing, err := session(t).CallTool(ctx, &mcp.CallToolParams{Name: "screenshot_route", Arguments: map[string]any{"path": "/no-such-page"}})
	if err != nil {
		t.Fatal(err)
	}
	if !missing.IsError || !strings.Contains(textOf(missing), "404") {
		t.Fatalf("a missing page answered: error=%v %s", missing.IsError, textOf(missing))
	}
}

// TestREQ_AI_04_A11yAudit covers a11y_audit with the vendored axe-core: a
// fixture with an image that has no text alternative reports image-alt, and
// the home page reports no such fault (REQ-AI-04).
func TestREQ_AI_04_A11yAudit(t *testing.T) {
	type audit struct {
		Violations []struct {
			ID, Impact, Help string
			Nodes            []struct {
				Target []string
				HTML   string
			}
		}
	}
	var bad audit
	call(t, "a11y_audit", map[string]any{"component": "Bad", "name": "Default"}, &bad)
	var found bool
	for _, v := range bad.Violations {
		if v.ID == "image-alt" {
			found = true
			if v.Impact != "critical" || v.Help == "" || len(v.Nodes) != 1 || !strings.Contains(v.Nodes[0].HTML, "<img") {
				t.Fatalf("image-alt = %+v", v)
			}
		}
	}
	if !found {
		t.Fatalf("the audit did not find image-alt: %+v", bad.Violations)
	}

	var home audit
	call(t, "a11y_audit", map[string]any{"path": "/"}, &home)
	for _, v := range home.Violations {
		if v.ID == "image-alt" || v.Impact == "critical" {
			t.Fatalf("the home page has a critical fault: %+v", v)
		}
	}
}

// TestREQ_AI_04_Registry covers registry_search and registry_add: a query
// finds an item, and an add writes the item into the app (REQ-AI-04).
func TestREQ_AI_04_Registry(t *testing.T) {
	var search struct {
		Items []struct{ Name, Version, Description, Kind string }
	}
	call(t, "registry_search", map[string]any{"query": "badge"}, &search)
	if len(search.Items) == 0 || search.Items[0].Name != "badge" || search.Items[0].Version == "" {
		t.Fatalf("search badge = %+v", search)
	}
	call(t, "registry_search", map[string]any{"query": ""}, &search)
	if len(search.Items) < 40 {
		t.Fatalf("an empty query lists %d items", len(search.Items))
	}
	call(t, "registry_search", map[string]any{"query": "zzz-no-such-item"}, &search)
	if len(search.Items) != 0 {
		t.Fatalf("a query with no match = %+v", search)
	}

	var added struct {
		Added []struct {
			Name, Version string
			Files         []string
		}
	}
	call(t, "registry_add", map[string]any{"item": "badge"}, &added)
	if len(added.Added) == 0 || added.Added[len(added.Added)-1].Name != "badge" {
		t.Fatalf("registry_add = %+v", added)
	}
	if _, err := os.Stat(filepath.Join(shared.dir, "ui", "badge", "Badge.gx")); err != nil {
		t.Fatalf("registry_add wrote no component: %v", err)
	}
	// The new component is in the model, and its fixture renders.
	var out struct{ HTML string }
	call(t, "render_fixture", map[string]any{"component": "Badge", "name": "Default"}, &out)
	if !strings.Contains(out.HTML, "data-slot=\"badge\"") {
		t.Fatalf("the added badge does not render:\n%s", out.HTML)
	}
}
