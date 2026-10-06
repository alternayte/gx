package gx

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Island returns the element of a TypeScript island (REQ-ISL-01). name is
// the import path of the package and the component name. props is the JSON
// of the props. The generated component function of an island calls it.
//
// The src attribute names the entry file of the island in the bundle that
// gx.SetIslands installed (REQ-ISL-03). In dev, an island that the bundle
// does not hold is a panic, because the browser cannot mount it.
func Island(name, props string) Node {
	attrs := Attrs{
		{Key: "name", Value: name},
		{Key: "props", Value: props},
	}
	if src := islandSrc(name); src != "" {
		attrs = append(attrs, Attr{Key: "src", Value: src, Kind: AttrURL})
	} else if devMode.Load() {
		panic("gx: the island " + name + " is not in the installed bundle; call gx.SetIslands(gxislands.Bundle()) in main")
	}
	// The island renders into the root. A morph skips an element with
	// data-ignore-morph on both sides, so it keeps what the island put
	// there, and it still updates the props of the element around it
	// (REQ-ISL-06).
	return El(islandElement, attrs, El("div", Attrs{
		Bool("data-gx-island-root", true),
		Bool("data-ignore-morph", true),
	}))
}

// islandElement is the custom element of an island.
const islandElement = "gx-island"

// The AppendJSON functions write the props of an island with no reflection
// (REQ-ISL-02). The generated encoder of an island calls them.

const hexDigits = "0123456789abcdef"

// AppendJSONString appends s as a JSON string. It escapes <, > and & so the
// text is safe in a script, and writes an invalid UTF-8 byte as U+FFFD.
func AppendJSONString(b []byte, s string) []byte {
	b = append(b, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				i++
				continue
			}
			b = append(b, s[start:i]...)
			switch c {
			case '"', '\\':
				b = append(b, '\\', c)
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			case '\b':
				b = append(b, '\\', 'b')
			case '\f':
				b = append(b, '\\', 'f')
			default:
				b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', 'f', 'f', 'f', 'd')
			i += size
			start = i
		case r == 0x2028 || r == 0x2029:
			// JavaScript before ES2019 reads these as line ends.
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', '2', '0', '2', hexDigits[r&0xf])
			i += size
			start = i
		default:
			i += size
		}
	}
	b = append(b, s[start:]...)
	return append(b, '"')
}

// AppendJSONSignalRef appends a gx.SignalRef prop of an island
// (REQ-ISL-04). The island loader turns the object into a reference that
// the context of the island resolves.
//
// path is the bracket path of a SignalRef, for example
// ["cart"]["Cart"]["qty"]; the props hold its parts as a list.
func AppendJSONSignalRef(b []byte, path string) []byte {
	b = append(b, `{"$signal":[`...)
	for i := 0; strings.HasPrefix(path, "["); i++ {
		quoted, err := strconv.QuotedPrefix(path[1:])
		if err != nil {
			break
		}
		part, err := strconv.Unquote(quoted)
		if err != nil {
			break
		}
		if i > 0 {
			b = append(b, ',')
		}
		b = AppendJSONString(b, part)
		path = strings.TrimPrefix(path[1+len(quoted):], "]")
	}
	return append(b, ']', '}')
}

// AppendJSONBool appends a JSON boolean.
func AppendJSONBool(b []byte, v bool) []byte { return strconv.AppendBool(b, v) }

// AppendJSONInt appends a JSON number. In dev it panics for an integer
// outside the 53-bit safe range, because the browser rounds it.
func AppendJSONInt(b []byte, v int64) []byte {
	if devMode.Load() && (v > maxSafeInt || v < -maxSafeInt) {
		panic("gx: an integer in an island prop is outside the 53-bit safe range of JavaScript: " + strconv.FormatInt(v, 10))
	}
	return strconv.AppendInt(b, v, 10)
}

// AppendJSONUint appends a JSON number. In dev it panics for an integer
// outside the 53-bit safe range, because the browser rounds it.
func AppendJSONUint(b []byte, v uint64) []byte {
	if devMode.Load() && v > maxSafeInt {
		panic("gx: an integer in an island prop is outside the 53-bit safe range of JavaScript: " + strconv.FormatUint(v, 10))
	}
	return strconv.AppendUint(b, v, 10)
}

// AppendJSONFloat appends a JSON number in the form encoding/json writes.
// JSON has no NaN and no infinity: such a value is null, and a panic in dev.
func AppendJSONFloat(b []byte, f float64) []byte {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if devMode.Load() {
			panic("gx: an island prop holds " + strconv.FormatFloat(f, 'g', -1, 64) + ", which JSON cannot hold")
		}
		return append(b, "null"...)
	}
	abs := math.Abs(f)
	format := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	b = strconv.AppendFloat(b, f, format, -1, 64)
	if format == 'e' {
		// e-09 becomes e-9, as ES6 writes it.
		if n := len(b); n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	return b
}

// AppendJSONTime appends a time as a JSON string in RFC 3339 form, which
// the Date constructor of JavaScript reads.
func AppendJSONTime(b []byte, t time.Time) []byte {
	b = append(b, '"')
	b = t.AppendFormat(b, time.RFC3339Nano)
	return append(b, '"')
}

// SortedKeys returns the keys of a map in order, so that the props of an
// island render the same bytes each time.
func SortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// IsZero reports whether v is the zero value of its type. The generated
// encoder of an island calls it for a json omitzero option.
func IsZero[T comparable](v T) bool {
	var zero T
	return v == zero
}
