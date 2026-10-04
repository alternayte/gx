// Package gxconfig reads the hand-written gx.toml of an app. The 0.1.0
// shape is small: string keys in tables, one per line. Downloads read the
// [mirrors] table (REQ-STY-12).
package gxconfig

import (
	"os"
	"path/filepath"
	"strings"
)

// Config is the parsed gx.toml.
type Config struct {
	Mirrors map[string]string
}

// Load reads root/gx.toml. A missing file is an empty config.
func Load(root string) (Config, error) {
	cfg := Config{Mirrors: map[string]string{}}
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
		if section == "mirrors" {
			cfg.Mirrors[key] = value
		}
	}
	return cfg, nil
}

// Mirror returns the mirror of a source, or "".
func (c Config) Mirror(source string) string { return c.Mirrors[source] }

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
