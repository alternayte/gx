// Package gxconfig reads the hand-written gx.toml of an app. The 0.1.0
// shape is small: string keys in tables, one per line. Downloads read the
// [mirrors] table (REQ-STY-12); the export reads the [site] table
// (REQ-CNT-09).
package gxconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The adapter names of gx.toml (REQ-ACT-09).
const (
	AdapterDatastar = "datastar"
	AdapterHtmx     = "htmx"
)

// Config is the parsed gx.toml.
type Config struct {
	// Adapter is the adapter key at the top of the file: "datastar" or
	// "htmx". Empty means Datastar. The compiler reads it, because signals
	// and client expressions are compile errors under htmx (REQ-ACT-09).
	// It must name the adapter that gx.Config.Adapter holds.
	Adapter    string
	Mirrors    map[string]string
	Site       Site
	Registry   Registry
	Registries map[string]RegistrySource
	// IslandRoots are directories outside the app whose islands the app
	// bundles too, relative to the app: the `roots` key of [islands], with
	// white space between two directories. An app that imports component
	// packages of a sibling directory names that directory here.
	IslandRoots []string
	Widgets     Widgets
	// RouteSheets is the largest number of distinct stylesheets that the
	// build makes for the page routes: the `route_sheets` key of [styles]
	// (REQ-STY-13). Each one is one run of Tailwind. Above the number,
	// each page links the stylesheet of the app. Zero is the default,
	// DefaultRouteSheets; a negative number turns the stylesheets of the
	// routes off.
	RouteSheets int
	// Budget holds the size limits of the page routes (REQ-DEV-13).
	Budget Budget
}

// Budget is the [budget] table of gx.toml: the largest gzipped size, in
// bytes, of the JS and of the CSS that one page route loads (REQ-DEV-13).
// Zero is no limit. A table [budget.routes."GET /basket"] sets the two
// numbers of one route.
type Budget struct {
	JS, CSS int
	Routes  map[string]RouteBudget
}

// RouteBudget is the budget of one route pattern. A number that is not set
// is -1: the route then has the default of the [budget] table.
type RouteBudget struct {
	JS, CSS int
}

// Set reports whether the app has a budget.
func (b Budget) Set() bool { return b.JS > 0 || b.CSS > 0 || len(b.Routes) > 0 }

// DefaultRouteSheets is the limit of distinct route stylesheets of an app
// that sets none.
const DefaultRouteSheets = 16

// Widgets is the [widgets] table of gx.toml: the npm package of the widgets
// of the app (REQ-ISL-13, REQ-ISL-14).
type Widgets struct {
	// Name is the name of the npm package, for example "@acme/widgets".
	Name string
	// Version is the version of the contract of the widgets with a host.
	// `gx wc check` compares it with the version of the baseline.
	Version string
	// Server is the origin of the Gx server, and Base the base path of the
	// app on it. The element files of the package call them.
	Server string
	Base   string
	// Registry is the URL of the npm registry. Empty is the public one.
	Registry string
	// Access is "public" or "restricted". Empty is the rule of the
	// registry.
	Access string
}

// Registry is the [registry] table of gx.toml (REQ-REG-02). URL is the
// default registry base, an http(s) URL or a directory. Dir maps the
// published target root to an app directory (default "ui").
type Registry struct {
	URL string
	Dir string
}

// RegistrySource is one named [registries.<name>] entry (REQ-REG-04).
// Headers are "name: value" strings sent with every request to an HTTP
// registry.
type RegistrySource struct {
	URL     string
	Headers []string
}

// Site is the [site] table of gx.toml (REQ-CNT-09).
type Site struct {
	// URL is the canonical site root, for example "https://docs.example.com".
	// Empty disables canonical links and sitemap.xml.
	URL string
	// Title is the site name used in the page titles.
	Title string
	// TitleTemplate formats the page title with %s, for example
	// "%s | Deedbox docs". Empty uses "<page> | <site>".
	TitleTemplate string
	// Description is the default page description.
	Description string
}

// pluginAdapters are the adapter names that the plugins of the project add
// (REQ-PLG-01). The gx command sets them before a command runs.
var pluginAdapters = map[string]bool{}

// SetPluginAdapters gives the adapter names of the plugins of the project.
// It replaces the names of an earlier call.
func SetPluginAdapters(names []string) {
	next := make(map[string]bool, len(names))
	for _, name := range names {
		next[name] = true
	}
	pluginAdapters = next
}

// Load reads root/gx.toml. A missing file is an empty config.
func Load(root string) (Config, error) {
	cfg := Config{Mirrors: map[string]string{}, Registries: map[string]RegistrySource{}}
	data, err := os.ReadFile(filepath.Join(root, "gx.toml"))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(stripComment(line))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if unquoted, ok := unquote(value); ok {
			value = unquoted
		}
		if pattern, ok := strings.CutPrefix(section, "budget.routes."); ok {
			// [budget.routes."GET /basket"]: the pattern is a quoted key.
			if unquoted, ok := unquote(strings.TrimSpace(pattern)); ok {
				pattern = unquoted
			}
			if cfg.Budget.Routes == nil {
				cfg.Budget.Routes = map[string]RouteBudget{}
			}
			route, have := cfg.Budget.Routes[pattern]
			if !have {
				route = RouteBudget{JS: -1, CSS: -1}
			}
			if n, err := strconv.Atoi(value); err == nil && n >= 0 {
				switch key {
				case "js":
					route.JS = n
				case "css":
					route.CSS = n
				}
			}
			cfg.Budget.Routes[pattern] = route
			continue
		}
		switch section {
		case "budget":
			if n, err := strconv.Atoi(value); err == nil && n >= 0 {
				switch key {
				case "js":
					cfg.Budget.JS = n
				case "css":
					cfg.Budget.CSS = n
				}
			}
		case "":
			if key == "adapter" {
				if value != AdapterDatastar && value != AdapterHtmx && !pluginAdapters[value] {
					return cfg, fmt.Errorf("gx.toml: unknown adapter %q; the adapters are %q and %q", value, AdapterDatastar, AdapterHtmx)
				}
				cfg.Adapter = value
			}
		case "mirrors":
			cfg.Mirrors[key] = value
		case "registry":
			switch key {
			case "url":
				cfg.Registry.URL = value
			case "dir":
				cfg.Registry.Dir = value
			}
		case "widgets":
			switch key {
			case "name":
				cfg.Widgets.Name = value
			case "version":
				cfg.Widgets.Version = value
			case "server":
				cfg.Widgets.Server = value
			case "base":
				cfg.Widgets.Base = value
			case "registry":
				cfg.Widgets.Registry = value
			case "access":
				cfg.Widgets.Access = value
			}
		case "islands":
			if key == "roots" {
				cfg.IslandRoots = strings.Fields(value)
			}
		case "styles":
			if key == "route_sheets" {
				if n, err := strconv.Atoi(value); err == nil {
					cfg.RouteSheets = n
				}
			}
		case "registries":
			source := cfg.Registries[key]
			source.URL = value
			cfg.Registries[key] = source
		case "site":
			switch key {
			case "url":
				cfg.Site.URL = value
			case "title":
				cfg.Site.Title = value
			case "title_template":
				cfg.Site.TitleTemplate = value
			case "description":
				cfg.Site.Description = value
			}
		default:
			if name, ok := strings.CutPrefix(section, "registries."); ok {
				source := cfg.Registries[name]
				switch key {
				case "url":
					source.URL = value
				case "header", "headers":
					for _, h := range strings.Split(value, ";") {
						if h = strings.TrimSpace(h); h != "" {
							source.Headers = append(source.Headers, h)
						}
					}
				}
				cfg.Registries[name] = source
			}
		}
	}
	return cfg, nil
}

// Mirror returns the mirror of a source, or "".
func (c Config) Mirror(source string) string { return c.Mirrors[source] }

// PageTitle returns the document title of one page (REQ-CNT-09).
func (s Site) PageTitle(title string) string {
	if title == "" {
		return s.Title
	}
	if s.TitleTemplate != "" {
		return strings.ReplaceAll(s.TitleTemplate, "%s", title)
	}
	if s.Title == "" {
		return title
	}
	return title + " | " + s.Title
}

// stripComment removes a # comment that is not inside a quoted string.
func stripComment(line string) string {
	inQuote := byte(0)
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"', '\'':
			if inQuote == 0 {
				inQuote = line[i]
			} else if inQuote == line[i] {
				inQuote = 0
			}
		case '#':
			if inQuote == 0 {
				return line[:i]
			}
		}
	}
	return line
}

// unquote removes surrounding double quotes and simple escapes.
func unquote(value string) (string, bool) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value, false
	}
	out := value[1 : len(value)-1]
	out = strings.ReplaceAll(out, `\"`, `"`)
	out = strings.ReplaceAll(out, `\\`, `\`)
	return out, true
}
