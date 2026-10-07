// Package gxconfig reads the hand-written gx.toml of an app. The 0.1.0
// shape is small: string keys in tables, one per line. Downloads read the
// [mirrors] table (REQ-STY-12); the export reads the [site] table
// (REQ-CNT-09).
package gxconfig

import (
	"fmt"
	"os"
	"path/filepath"
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
		switch section {
		case "":
			if key == "adapter" {
				if value != AdapterDatastar && value != AdapterHtmx {
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
		case "islands":
			if key == "roots" {
				cfg.IslandRoots = strings.Fields(value)
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
