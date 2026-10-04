package exporter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/alternayte/gx/internal/images"
)

// imgTag finds one img element in the rendered page.
var imgTag = regexp.MustCompile(`(?is)<img\b[^>]*>`)

// imgAttr finds one attribute of an img element.
var imgAttr = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*"([^"]*)"`)

// imageWriter copies content images with content hashes and writes the
// resized srcset variants (REQ-CNT-11).
type imageWriter struct {
	dir  string
	out  string
	seen map[string]string
}

// rewriteImages rewrites every local img of a page (REQ-CNT-11).
func (w *imageWriter) rewriteImages(body []byte) []byte {
	return imgTag.ReplaceAllFunc(body, func(tag []byte) []byte {
		attrs := parseImgAttrs(string(tag))
		src := attrs.get("src")
		if src == "" || strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "data:") {
			return tag
		}
		file, ok := resolveImage(w.dir, src)
		if !ok {
			return tag
		}
		hashed, width, height, srcset, ok := w.writeImage(file, src)
		if !ok {
			return tag
		}
		attrs.set("src", hashed)
		attrs.set("width", strconv.Itoa(width))
		attrs.set("height", strconv.Itoa(height))
		attrs.set("loading", "lazy")
		attrs.set("decoding", "async")
		if srcset != "" {
			attrs.set("srcset", srcset)
		}
		return []byte(attrs.render())
	})
}

// writeImage copies one image and its variants and returns the hashed URL,
// the pixel size and the srcset.
func (w *imageWriter) writeImage(file, src string) (string, int, int, string, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", 0, 0, "", false
	}
	width, height, _, err := images.Dimensions(data)
	if err != nil {
		return "", 0, 0, "", false
	}
	if w.seen == nil {
		w.seen = map[string]string{}
	}
	key := file
	if hashed, ok := w.seen[key]; ok {
		srcset := w.seen[src+"#srcset"]
		return hashed, width, height, srcset, true
	}
	rel := strings.TrimPrefix(src, "/")
	base := strings.TrimSuffix(rel, filepath.Ext(rel))
	format := images.FromPath(src)
	if format == "" {
		return "", 0, 0, "", false
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])[:8]
	hashedRel := fmt.Sprintf("%s.%s%s", base, hash, format.Ext())
	if err := writeOutput(w.out, hashedRel, data); err != nil {
		return "", 0, 0, "", false
	}
	var srcset []string
	for _, vw := range images.SrcsetWidths(width) {
		variant, _, err := images.Resize(data, vw)
		if err != nil {
			continue
		}
		variantRel := fmt.Sprintf("%s.%s.%dw%s", base, hash, vw, format.Ext())
		if err := writeOutput(w.out, variantRel, variant); err != nil {
			continue
		}
		srcset = append(srcset, "/"+variantRel+" "+strconv.Itoa(vw)+"w")
	}
	value := ""
	if len(srcset) > 0 {
		value = strings.Join(srcset, ", ")
	}
	w.seen[key] = "/" + hashedRel
	w.seen[src+"#srcset"] = value
	return "/" + hashedRel, width, height, value, true
}

// writeOutput writes one file below the output root.
func writeOutput(root, rel string, data []byte) error {
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

// resolveImage finds the source file of a local image URL. Images live
// under public/, static/ or the module root.
func resolveImage(dir, src string) (string, bool) {
	rel := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(src, "/")))
	if rel == "" || strings.HasPrefix(rel, "..") {
		return "", false
	}
	for _, root := range []string{"public", "static", ""} {
		full := filepath.Join(dir, root, rel)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full, true
		}
	}
	return "", false
}

// imgAttrs holds the attributes of one img element in source order.
type imgAttrs struct {
	order []string
	vals  map[string]string
}

// parseImgAttrs reads the attributes of one img element.
func parseImgAttrs(tag string) *imgAttrs {
	out := &imgAttrs{vals: map[string]string{}}
	for _, m := range imgAttr.FindAllStringSubmatch(tag, -1) {
		key := strings.ToLower(m[1])
		if _, ok := out.vals[key]; !ok {
			out.order = append(out.order, key)
		}
		out.vals[key] = m[2]
	}
	return out
}

// get returns one attribute value.
func (a *imgAttrs) get(key string) string { return a.vals[key] }

// set writes one attribute value.
func (a *imgAttrs) set(key, value string) {
	if _, ok := a.vals[key]; !ok {
		a.order = append(a.order, key)
	}
	a.vals[key] = value
}

// render returns the img element with its attributes in source order.
func (a *imgAttrs) render() string {
	var b strings.Builder
	b.WriteString("<img")
	for _, key := range a.order {
		fmt.Fprintf(&b, ` %s="%s"`, key, escapeAttr(a.vals[key]))
	}
	b.WriteString(">")
	return b.String()
}

// escapeAttr escapes one attribute value.
func escapeAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
