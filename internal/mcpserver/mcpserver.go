// Package mcpserver is the dev MCP server of `gx mcp` (REQ-AI-04). An agent
// reads the app model, checks the app, renders fixtures and pages in
// headless Chrome, audits them with axe-core and installs registry items.
// It needs no node: Chrome runs through chromedp and axe-core is embedded.
package mcpserver

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alternayte/gx/internal/appmodel"
	"github.com/alternayte/gx/internal/apprun"
	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/registry"
	"github.com/alternayte/gx/internal/tscheck"
)

// axeSource is axe-core 4.13.0 (MPL-2.0, see axe.LICENSE). The audit runs
// it inside the page; the test pins its hash.
//
//go:embed axe.min.js
var axeSource string

// AxeVersion and AxeSHA256 pin the embedded axe-core build.
const (
	AxeVersion = "4.13.0"
	AxeSHA256  = "c24f097bd2f451d4f933e8bc7d8d539f8672a2ebcb5cc9f9f3eec8ca9470a0c1"
)

// SumAxe returns the hex sha256 of an axe-core build.
func SumAxe(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Options configure one server.
type Options struct {
	// Dir is the app module root.
	Dir string
	// Main is the app main package; apprun.DetectMain finds it when empty.
	Main string
	// Version is the Gx version the server reports.
	Version string
	// Installer returns the registry installer of the app, from gx.toml.
	Installer func() (registry.Installer, error)
	// Extra are the tools that the plugins of the project add
	// (REQ-PLG-01).
	Extra []ExtraTool
}

// ExtraTool is one tool of a plugin in the dev MCP server.
type ExtraTool struct {
	Name        string
	Description string
	// InputSchema is the JSON Schema of the arguments: an object schema.
	InputSchema json.RawMessage
	// Call gets the JSON object of the arguments and returns the text of
	// the answer.
	Call func(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolNames are the names of the tools of the dev MCP server. A plugin
// cannot take one.
var ToolNames = []string{"describe", "check", "routes", "render_fixture", "screenshot_route", "a11y_audit", "registry_search", "registry_add"}

// Server holds the state the tools share: one running dev build of the app
// and one headless Chrome. Both start on the first tool call that needs
// them.
type Server struct {
	opt Options
	mcp *mcp.Server

	mu       sync.Mutex
	app      *apprun.App
	snapshot string
	browser  context.Context
	closers  []context.CancelFunc
}

// New returns a server for the app in opt.Dir.
func New(opt Options) *Server {
	if abs, err := filepath.Abs(opt.Dir); err == nil {
		opt.Dir = abs
	}
	s := &Server{opt: opt}
	s.mcp = mcp.NewServer(&mcp.Implementation{Name: "gx", Title: "Gx dev tools", Version: opt.Version}, nil)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "describe",
		Description: "Return the app model: every component with its props, signals, fragments and fixtures, every route, action and form with its rules, the transitions, the icon sets and the installed registry items.",
	}, s.describe)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "check",
		Description: "Check the app like `gx check`: type errors, unknown props, broken links and stale generated code. Each diagnostic has a code, a position, a message, a doc link and, where known, a fix.",
	}, s.check)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "routes",
		Description: "List the routes of the app: method and pattern, route type, input fields, page, mount prefix, layouts and middleware.",
	}, s.routes)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "render_fixture",
		Description: "Render one fixture of one component in the dev build of the app. The format html returns the markup; the format png returns a screenshot from headless Chrome. `describe` lists the fixture names.",
	}, s.renderFixture)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "screenshot_route",
		Description: "Open one path of the running app in headless Chrome and return a PNG screenshot of the full page.",
	}, s.screenshotRoute)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "a11y_audit",
		Description: "Audit one page or one component fixture with axe-core in headless Chrome and return the accessibility violations with their impact, help text and failing elements.",
	}, s.a11yAudit)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "registry_search",
		Description: "Search the component registry of the app by name, kind or description. An empty query lists every item.",
	}, s.registrySearch)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "registry_add",
		Description: "Install one registry item and its dependencies into the app, like `gx add`. The files are copied into the app; an existing file with other content stops the install.",
	}, s.registryAdd)
	for _, extra := range opt.Extra {
		s.mcp.AddTool(&mcp.Tool{Name: extra.Name, Description: extra.Description, InputSchema: extra.InputSchema},
			func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var args json.RawMessage
				if req.Params != nil {
					args = req.Params.Arguments
				}
				text, err := extra.Call(ctx, args)
				if err != nil {
					return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
			})
	}
	return s
}

// MCP returns the protocol server, for a transport.
func (s *Server) MCP() *mcp.Server { return s.mcp }

// Run serves one client on the transport until it disconnects, then stops
// the app and Chrome.
func Run(ctx context.Context, opt Options, t mcp.Transport) error {
	s := New(opt)
	defer s.Close()
	return s.mcp.Run(ctx, t)
}

// Close stops the app and Chrome.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.closers) - 1; i >= 0; i-- {
		s.closers[i]()
	}
	s.closers, s.browser = nil, nil
	s.app.Stop()
	s.app = nil
}

// --- model tools ---

type noInput struct{}

// DescribeOutput is the result of describe.
type DescribeOutput struct {
	Model *compiler.AppModel `json:"model"`
}

func (s *Server) describe(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, DescribeOutput, error) {
	model, diags, err := appmodel.Describe(s.opt.Dir)
	if err != nil {
		return nil, DescribeOutput{}, err
	}
	if len(diags) > 0 {
		lines := make([]string, 0, len(diags)+1)
		lines = append(lines, "The app has diagnostics, so it has no model. Call check, fix them, then call describe again.")
		for _, d := range diags {
			lines = append(lines, d.String())
		}
		return nil, DescribeOutput{}, errors.New(strings.Join(lines, "\n"))
	}
	return nil, DescribeOutput{Model: model}, nil
}

// Diagnostic is one diagnostic in the result of check (REQ-AI-02).
type Diagnostic struct {
	Code    string `json:"code"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
	Doc     string `json:"doc"`
}

// CheckOutput is the result of check.
type CheckOutput struct {
	OK          bool         `json:"ok"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (s *Server) check(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, CheckOutput, error) {
	// The same check as `gx check`, with the TypeScript check of the
	// islands (REQ-ISL-08).
	diags, err := tscheck.App(ctx, s.opt.Dir, compiler.CheckOptions{})
	if err != nil {
		return nil, CheckOutput{}, err
	}
	out := CheckOutput{OK: len(diags) == 0, Diagnostics: []Diagnostic{}}
	for _, d := range diags {
		out.Diagnostics = append(out.Diagnostics, Diagnostic{
			Code: d.Code, File: d.File, Line: d.Line, Column: d.Col, Message: d.Msg, Fix: d.Fix, Doc: d.Doc(),
		})
	}
	return nil, out, nil
}

// RoutesOutput is the result of routes.
type RoutesOutput struct {
	Routes []compiler.RouteReport `json:"routes"`
}

func (s *Server) routes(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, RoutesOutput, error) {
	reports, diags := compiler.Routes(s.opt.Dir)
	if len(diags) > 0 {
		return nil, RoutesOutput{}, errors.New("the app has diagnostics; call check: " + diags[0].String())
	}
	if reports == nil {
		reports = []compiler.RouteReport{}
	}
	return nil, RoutesOutput{Routes: reports}, nil
}

// --- browser tools ---

// FixtureInput names one fixture.
type FixtureInput struct {
	Component string `json:"component" jsonschema:"the component name, for example Button"`
	Name      string `json:"name" jsonschema:"the fixture name, for example Default"`
	Package   string `json:"package,omitempty" jsonschema:"the import path of the component package; needed only when two packages hold the component name"`
	Format    string `json:"format,omitempty" jsonschema:"html (the default) or png"`
	Theme     string `json:"theme,omitempty" jsonschema:"light or dark; empty follows the system"`
}

// FixtureOutput is the result of render_fixture.
type FixtureOutput struct {
	Component string `json:"component"`
	Name      string `json:"name"`
	Format    string `json:"format"`
	// HTML is the markup of the fixture, for the format html.
	HTML string `json:"html,omitempty"`
	// Bytes is the size of the PNG, for the format png.
	Bytes int `json:"bytes,omitempty"`
}

func (s *Server) renderFixture(ctx context.Context, _ *mcp.CallToolRequest, in FixtureInput) (*mcp.CallToolResult, FixtureOutput, error) {
	format := strings.ToLower(in.Format)
	if format == "" {
		format = "html"
	}
	if format != "html" && format != "png" {
		return nil, FixtureOutput{}, fmt.Errorf("format %q: render_fixture has html and png", in.Format)
	}
	app, err := s.ensureApp(ctx)
	if err != nil {
		return nil, FixtureOutput{}, err
	}
	target := app.Base + fixturePath(in.Component, in.Name, in.Package, in.Theme)
	page, status, err := get(ctx, target)
	if err != nil {
		return nil, FixtureOutput{}, err
	}
	if status != http.StatusOK {
		return nil, FixtureOutput{}, s.noFixture(in.Component, in.Name)
	}
	out := FixtureOutput{Component: in.Component, Name: in.Name, Format: format}
	if format == "html" {
		out.HTML = fixtureMarkup(page)
		return nil, out, nil
	}
	shot, err := s.screenshot(ctx, target, 0, 0)
	if err != nil {
		return nil, FixtureOutput{}, err
	}
	out.Bytes = len(shot)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: shot, MIMEType: "image/png"}}}, out, nil
}

// fixturePath is the dev route of one fixture document.
func fixturePath(component, name, pkg, theme string) string {
	q := url.Values{"component": {component}, "name": {name}}
	if pkg != "" {
		q.Set("package", pkg)
	}
	if theme != "" {
		q.Set("theme", theme)
	}
	return "/_gx/gallery/fixture?" + q.Encode()
}

// fixtureMarkup cuts the fixture out of its document.
func fixtureMarkup(page string) string {
	start := strings.Index(page, `<main id="gx-fixture"`)
	if start < 0 {
		return page
	}
	open := strings.Index(page[start:], ">")
	end := strings.LastIndex(page, "</main>")
	if open < 0 || end < start+open {
		return page
	}
	return strings.TrimSpace(page[start+open+1 : end])
}

// noFixture names the fixtures that exist, so the caller can correct the
// name without a second tool.
func (s *Server) noFixture(component, name string) error {
	msg := fmt.Sprintf("no fixture %q of component %q.", name, component)
	model, diags, err := appmodel.Describe(s.opt.Dir)
	if err != nil || len(diags) > 0 {
		return errors.New(msg)
	}
	var known []string
	for _, c := range model.Components {
		if c.Name == component {
			known = append(known, c.Fixtures...)
		}
	}
	if len(known) == 0 {
		return errors.New(msg + " The component has no fixtures; add " + component + ".fixtures.go.")
	}
	return errors.New(msg + " Its fixtures are: " + strings.Join(known, ", ") + ".")
}

// RouteInput names one page of the app.
type RouteInput struct {
	Path   string `json:"path" jsonschema:"the site path with its query, for example /products/1?tab=reviews"`
	Width  int    `json:"width,omitempty" jsonschema:"the viewport width in CSS pixels; the default is 1280"`
	Height int    `json:"height,omitempty" jsonschema:"the viewport height in CSS pixels; the default is 800"`
}

// ScreenshotOutput is the result of screenshot_route.
type ScreenshotOutput struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

func (s *Server) screenshotRoute(ctx context.Context, _ *mcp.CallToolRequest, in RouteInput) (*mcp.CallToolResult, ScreenshotOutput, error) {
	app, err := s.ensureApp(ctx)
	if err != nil {
		return nil, ScreenshotOutput{}, err
	}
	target, err := pageURL(ctx, app, in.Path)
	if err != nil {
		return nil, ScreenshotOutput{}, err
	}
	shot, err := s.screenshot(ctx, target, in.Width, in.Height)
	if err != nil {
		return nil, ScreenshotOutput{}, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: shot, MIMEType: "image/png"}}},
		ScreenshotOutput{Path: in.Path, Bytes: len(shot)}, nil
}

// pageURL returns the address of a site path and fails when the app does
// not answer it with 200: a screenshot of an error page would mislead.
func pageURL(ctx context.Context, app *apprun.App, path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("path %q: write a site path that starts with /", path)
	}
	target := app.Base + path
	_, status, err := get(ctx, target)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("%s answers %d %s", path, status, http.StatusText(status))
	}
	return target, nil
}

// AuditInput names a page or a fixture.
type AuditInput struct {
	Path      string `json:"path,omitempty" jsonschema:"the site path of a page; give this or a component and a name"`
	Component string `json:"component,omitempty" jsonschema:"the component of a fixture"`
	Name      string `json:"name,omitempty" jsonschema:"the fixture name"`
	Package   string `json:"package,omitempty" jsonschema:"the import path of the component package; needed only when two packages hold the component name"`
	Theme     string `json:"theme,omitempty" jsonschema:"light or dark, for a fixture; empty follows the system"`
}

// AuditNode is one element that fails a rule.
type AuditNode struct {
	Target  []string `json:"target"`
	HTML    string   `json:"html"`
	Summary string   `json:"summary"`
}

// Violation is one failed axe rule.
type Violation struct {
	ID      string      `json:"id"`
	Impact  string      `json:"impact"`
	Help    string      `json:"help"`
	HelpURL string      `json:"helpUrl"`
	Nodes   []AuditNode `json:"nodes"`
}

// AuditOutput is the result of a11y_audit.
type AuditOutput struct {
	Violations []Violation `json:"violations"`
	// Passes is the number of rules that passed.
	Passes int    `json:"passes"`
	Axe    string `json:"axe"`
}

// auditScript runs axe on the document and returns JSON text.
const auditScript = `(async () => {
  const r = await axe.run(document);
  return JSON.stringify({
    passes: r.passes.length,
    violations: r.violations.map((v) => ({
      id: v.id, impact: v.impact || "", help: v.help, helpUrl: v.helpUrl,
      nodes: v.nodes.map((n) => ({ target: n.target.map(String), html: n.html, summary: n.failureSummary || "" })),
    })),
  });
})()`

func (s *Server) a11yAudit(ctx context.Context, _ *mcp.CallToolRequest, in AuditInput) (*mcp.CallToolResult, AuditOutput, error) {
	app, err := s.ensureApp(ctx)
	if err != nil {
		return nil, AuditOutput{}, err
	}
	var target string
	switch {
	case in.Path != "" && in.Component == "":
		target, err = pageURL(ctx, app, in.Path)
		if err != nil {
			return nil, AuditOutput{}, err
		}
	case in.Path == "" && in.Component != "" && in.Name != "":
		target = app.Base + fixturePath(in.Component, in.Name, in.Package, in.Theme)
		if _, status, err := get(ctx, target); err != nil {
			return nil, AuditOutput{}, err
		} else if status != http.StatusOK {
			return nil, AuditOutput{}, s.noFixture(in.Component, in.Name)
		}
	default:
		return nil, AuditOutput{}, errors.New("give a path, or a component and a fixture name")
	}
	tab, cancel, err := s.tab(ctx)
	if err != nil {
		return nil, AuditOutput{}, err
	}
	defer cancel()
	var raw string
	await := func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }
	if err := chromedp.Run(tab,
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(target),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(axeSource, nil),
		chromedp.Evaluate(auditScript, &raw, await),
	); err != nil {
		return nil, AuditOutput{}, fmt.Errorf("audit in Chrome: %w", err)
	}
	out := AuditOutput{Axe: AxeVersion}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, AuditOutput{}, fmt.Errorf("axe result: %w", err)
	}
	if out.Violations == nil {
		out.Violations = []Violation{}
	}
	return nil, out, nil
}

// --- registry tools ---

// SearchInput is the input of registry_search.
type SearchInput struct {
	Query string `json:"query,omitempty" jsonschema:"words to find in the name, the kind or the description of an item; empty lists every item"`
}

// SearchOutput is the result of registry_search.
type SearchOutput struct {
	Items []registry.Summary `json:"items"`
}

func (s *Server) registrySearch(ctx context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	installer, err := s.installer()
	if err != nil {
		return nil, SearchOutput{}, err
	}
	index, err := installer.Index()
	if err != nil {
		return nil, SearchOutput{}, err
	}
	words := strings.Fields(strings.ToLower(in.Query))
	type hit struct {
		item registry.Summary
		rank int
	}
	var hits []hit
	for _, item := range index.Items {
		name := strings.ToLower(item.Name)
		text := name + " " + strings.ToLower(item.Kind) + " " + strings.ToLower(item.Description)
		rank, all := 0, true
		for _, w := range words {
			switch {
			case name == w:
				rank += 3
			case strings.Contains(name, w):
				rank += 2
			case strings.Contains(text, w):
				rank++
			default:
				all = false
			}
		}
		if all {
			hits = append(hits, hit{item, rank})
		}
	}
	// The best match first; the registry order breaks a tie.
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].rank > hits[j].rank })
	out := SearchOutput{Items: []registry.Summary{}}
	for _, h := range hits {
		out.Items = append(out.Items, h.item)
	}
	return nil, out, nil
}

// AddInput is the input of registry_add.
type AddInput struct {
	Item string `json:"item" jsonschema:"the item name, with an optional version: button or button@0.1.0"`
}

// AddedItem is one item registry_add installed.
type AddedItem struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

// AddOutput is the result of registry_add: the items in install order,
// dependencies first.
type AddOutput struct {
	Added []AddedItem `json:"added"`
}

func (s *Server) registryAdd(ctx context.Context, _ *mcp.CallToolRequest, in AddInput) (*mcp.CallToolResult, AddOutput, error) {
	installer, err := s.installer()
	if err != nil {
		return nil, AddOutput{}, err
	}
	name, version, _ := strings.Cut(in.Item, "@")
	items, err := installer.Add(strings.TrimSpace(name), strings.TrimSpace(version))
	if err != nil {
		return nil, AddOutput{}, err
	}
	out := AddOutput{Added: []AddedItem{}}
	for _, item := range items {
		added := AddedItem{Name: item.Name, Version: item.Version, Files: []string{}}
		for _, f := range item.Files {
			added.Files = append(added.Files, f.Target)
		}
		out.Added = append(out.Added, added)
	}
	return nil, out, nil
}

func (s *Server) installer() (*registry.Installer, error) {
	if s.opt.Installer == nil {
		return nil, errors.New("the app has no registry; set [registry] url in gx.toml")
	}
	installer, err := s.opt.Installer()
	if err != nil {
		return nil, err
	}
	return &installer, nil
}

// --- the app and Chrome ---

// ensureApp returns the running dev build of the app. It builds on the
// first call and again after a source file changed.
func (s *Server) ensureApp(ctx context.Context) (*apprun.App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app != nil && sourceSnapshot(s.opt.Dir) == s.snapshot {
		return s.app, nil
	}
	s.app.Stop()
	s.app = nil
	// The app outlives the tool call that started it.
	app, err := apprun.Start(context.WithoutCancel(ctx), s.opt.Dir, s.opt.Main)
	if err != nil {
		return nil, fmt.Errorf("the app does not build: %w", err)
	}
	s.app = app
	// The build writes generated files, so the snapshot is taken after it.
	s.snapshot = sourceSnapshot(s.opt.Dir)
	return app, nil
}

// sourceSnapshot hashes the name, size and time of every source file.
func sourceSnapshot(dir string) string {
	h := sha256.New()
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".gx", "node_modules", "dist", "bin", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".gx", ".css", ".md", ".markdown", ".toml", ".mod":
		default:
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

// tab opens a new tab of the shared headless Chrome.
func (s *Server) tab(ctx context.Context) (context.Context, context.CancelFunc, error) {
	s.mu.Lock()
	if s.browser == nil {
		alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:],
			// The first start of Chrome on a machine with load can take
			// longer than the 20 s default.
			chromedp.WSURLReadTimeout(90*time.Second))...)
		browser, cancelBrowser := chromedp.NewContext(alloc)
		if err := chromedp.Run(browser); err != nil {
			cancelBrowser()
			cancelAlloc()
			s.mu.Unlock()
			return nil, nil, fmt.Errorf("start headless Chrome: %w. Install Chrome or Chromium", err)
		}
		s.browser = browser
		s.closers = append(s.closers, cancelAlloc, cancelBrowser)
	}
	browser := s.browser
	s.mu.Unlock()
	tab, cancelTab := chromedp.NewContext(browser)
	tab, cancelTimeout := context.WithTimeout(tab, 60*time.Second)
	stop := context.AfterFunc(ctx, cancelTab)
	return tab, func() {
		stop()
		cancelTimeout()
		cancelTab()
	}, nil
}

// screenshot returns a PNG of the full page at target.
func (s *Server) screenshot(ctx context.Context, target string, width, height int) ([]byte, error) {
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 800
	}
	tab, cancel, err := s.tab(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	var shot []byte
	if err := chromedp.Run(tab,
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate(target),
		chromedp.WaitReady("body", chromedp.ByQuery),
		// Quality 100 gives a PNG.
		chromedp.FullScreenshot(&shot, 100),
	); err != nil {
		return nil, fmt.Errorf("screenshot in Chrome: %w", err)
	}
	return shot, nil
}

// get reads one address of the app.
func get(ctx context.Context, target string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", 0, err
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	return string(body), resp.StatusCode, nil
}

// Stdio is the transport of `gx mcp`: JSON-RPC on stdin and stdout.
func Stdio() mcp.Transport { return &mcp.StdioTransport{} }
