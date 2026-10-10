package gxstyles

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// TokensFile is the generated file with the theme tokens as Go values.
const TokensFile = "tokens_gx.go"

// TokensPath returns the path of the generated tokens file under root.
func TokensPath(root string) string { return filepath.Join(root, Dir, TokensFile) }

// ThemeValues returns the tokens of a theme file as values that an email
// client reads (REQ-STY-14): a colour is a hex colour and a length is a
// number of pixels. A token is a custom property of the `:root` rule, for
// the light theme, or of a `.dark` rule or a `:root` rule in
// `@media (prefers-color-scheme: dark)`, for the dark theme. The dark map
// holds each light token that the dark theme does not set. A colour with
// alpha is the colour on the `--background` of its theme, because an email
// client does not read alpha. A token that is no colour and no length is
// not in the maps.
func ThemeValues(css []byte) (light, dark map[string]string) {
	rawLight, rawDark := map[string]string{}, map[string]string{}
	scanTheme(css, false, rawLight, rawDark)
	for name, value := range rawLight {
		if _, ok := rawDark[name]; !ok {
			rawDark[name] = value
		}
	}
	return convertTokens(rawLight), convertTokens(rawDark)
}

// scanTheme walks the rules of one level of a theme file.
func scanTheme(css []byte, darkMedia bool, light, dark map[string]string) {
	for i := 0; i < len(css); {
		open := indexOutsideStrings(css, i, "{;}")
		if open < 0 {
			return
		}
		if css[open] != '{' {
			i = open + 1
			continue
		}
		end := closeOf(css, open)
		prelude := strings.Join(strings.Fields(stripComments(string(css[i:open]))), " ")
		body := css[open+1 : end]
		i = end + 1
		switch {
		case strings.HasPrefix(prelude, "@media"):
			scanTheme(body, darkMedia || (strings.Contains(prelude, "prefers-color-scheme") && strings.Contains(prelude, "dark")), light, dark)
		case strings.HasPrefix(prelude, "@layer"), strings.HasPrefix(prelude, "@supports"):
			scanTheme(body, darkMedia, light, dark)
		case strings.HasPrefix(prelude, "@"):
			// @theme, @keyframes and the others hold no theme token.
		default:
			for _, sel := range strings.Split(prelude, ",") {
				sel = strings.TrimSpace(sel)
				switch {
				case sel == ".dark" || strings.HasPrefix(sel, ":root.dark") || (darkMedia && strings.HasPrefix(sel, ":root")):
					declarations(body, dark)
				case sel == ":root" && !darkMedia:
					declarations(body, light)
				}
			}
		}
	}
}

func stripComments(s string) string {
	for {
		start := strings.Index(s, "/*")
		if start < 0 {
			return s
		}
		end := strings.Index(s[start+2:], "*/")
		if end < 0 {
			return s[:start]
		}
		s = s[:start] + " " + s[start+2+end+2:]
	}
}

// declarations puts the custom properties of one rule body into out.
func declarations(body []byte, out map[string]string) {
	for _, decl := range splitOutside([]byte(stripComments(string(body))), ';') {
		name, value, ok := strings.Cut(decl, ":")
		name = strings.TrimSpace(name)
		if ok && strings.HasPrefix(name, "--") {
			out[name] = strings.TrimSpace(value)
		}
	}
}

// convertTokens makes the values of one theme. A token whose value is one
// var() gets the value of that token.
func convertTokens(raw map[string]string) map[string]string {
	resolve := func(value string) string {
		for range 8 {
			m := varRef.FindStringSubmatch(value)
			if m == nil || !strings.HasPrefix(value, "var(") || !strings.HasSuffix(value, ")") {
				break
			}
			next, ok := raw[m[1]]
			if !ok {
				break
			}
			value = next
		}
		return value
	}
	background := [3]float64{1, 1, 1}
	if c, _, ok := parseColor(resolve(raw["--background"])); ok {
		background = c
	}
	out := map[string]string{}
	for name, value := range raw {
		value = resolve(value)
		if px, ok := parseLength(value); ok {
			out[name] = px
			continue
		}
		if c, alpha, ok := parseColor(value); ok {
			for i := range c {
				c[i] = c[i]*alpha + background[i]*(1-alpha)
			}
			out[name] = hex(c)
		}
	}
	return out
}

// parseLength reads a length in px, rem or em and returns it in pixels.
// One rem is 16 pixels.
func parseLength(value string) (string, bool) {
	for unit, factor := range map[string]float64{"px": 1, "rem": 16, "em": 16} {
		num, ok := strings.CutSuffix(value, unit)
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
		if err != nil {
			continue
		}
		return strconv.FormatFloat(math.Round(f*factor*100)/100, 'f', -1, 64) + "px", true
	}
	return "", false
}

func hex(c [3]float64) string {
	var b [3]int
	for i, v := range c {
		b[i] = int(math.Round(math.Min(1, math.Max(0, v)) * 255))
	}
	return fmt.Sprintf("#%02x%02x%02x", b[0], b[1], b[2])
}

// number reads a CSS number or a percentage. One hundred percent is full.
func number(s string, full float64) (float64, bool) {
	if p, ok := strings.CutSuffix(s, "%"); ok {
		f, err := strconv.ParseFloat(p, 64)
		return f / 100 * full, err == nil
	}
	if s == "none" {
		return 0, true
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// parseColor reads a hex colour, rgb(), hsl(), oklch(), oklab(), white or
// black. It returns the sRGB parts from 0 to 1 and the alpha.
func parseColor(value string) (c [3]float64, alpha float64, ok bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "white":
		return [3]float64{1, 1, 1}, 1, true
	case "black":
		return [3]float64{}, 1, true
	}
	if h, isHex := strings.CutPrefix(value, "#"); isHex {
		if len(h) == 3 || len(h) == 4 {
			long := ""
			for _, r := range h {
				long += string(r) + string(r)
			}
			h = long
		}
		if len(h) != 6 && len(h) != 8 {
			return c, 0, false
		}
		n, err := strconv.ParseUint(h, 16, 64)
		if err != nil {
			return c, 0, false
		}
		alpha = 1
		if len(h) == 8 {
			alpha = float64(n&0xff) / 255
			n >>= 8
		}
		return [3]float64{float64(n>>16&0xff) / 255, float64(n>>8&0xff) / 255, float64(n&0xff) / 255}, alpha, true
	}
	open := strings.Index(value, "(")
	if open < 0 || !strings.HasSuffix(value, ")") {
		return c, 0, false
	}
	fn := value[:open]
	args, alphaText, hasAlpha := strings.Cut(value[open+1:len(value)-1], "/")
	parts := strings.Fields(strings.ReplaceAll(args, ",", " "))
	if !hasAlpha && len(parts) == 4 {
		// The form with commas: rgba(0, 0, 0, 0.5).
		parts, alphaText, hasAlpha = parts[:3], parts[3], true
	}
	if len(parts) != 3 {
		return c, 0, false
	}
	alpha = 1
	if hasAlpha {
		if alpha, ok = number(strings.TrimSpace(alphaText), 1); !ok {
			return c, 0, false
		}
	}
	var v [3]float64
	full := map[string][3]float64{
		"rgb": {255, 255, 255}, "rgba": {255, 255, 255},
		"hsl": {360, 1, 1}, "hsla": {360, 1, 1},
		"oklch": {1, 0.4, 360}, "oklab": {1, 0.4, 0.4},
	}[fn]
	if full == [3]float64{} {
		return c, 0, false
	}
	for i, p := range parts {
		p = strings.TrimSuffix(p, "deg")
		if v[i], ok = number(p, full[i]); !ok {
			return c, 0, false
		}
	}
	switch fn {
	case "rgb", "rgba":
		return [3]float64{v[0] / 255, v[1] / 255, v[2] / 255}, alpha, true
	case "hsl", "hsla":
		return hslToRGB(v[0], v[1], v[2]), alpha, true
	case "oklch":
		h := v[2] * math.Pi / 180
		return oklabToRGB(v[0], v[1]*math.Cos(h), v[1]*math.Sin(h)), alpha, true
	}
	return oklabToRGB(v[0], v[1], v[2]), alpha, true
}

func hslToRGB(h, s, l float64) [3]float64 {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 30
	a := s * math.Min(l, 1-l)
	f := func(n float64) float64 {
		k := math.Mod(n+h, 12)
		return l - a*math.Max(-1, math.Min(math.Min(k-3, 9-k), 1))
	}
	return [3]float64{f(0), f(8), f(4)}
}

// oklabToRGB converts an Oklab colour to sRGB. A colour outside sRGB is
// clipped.
func oklabToRGB(L, a, b float64) [3]float64 {
	l := math.Pow(L+0.3963377774*a+0.2158037573*b, 3)
	m := math.Pow(L-0.1055613458*a-0.0638541728*b, 3)
	s := math.Pow(L-0.0894841775*a-1.2914855480*b, 3)
	linear := [3]float64{
		4.0767416621*l - 3.3077115913*m + 0.2309699292*s,
		-1.2684380046*l + 2.6097574011*m - 0.3413193965*s,
		-0.0041960863*l - 0.7034186147*m + 1.7076147010*s,
	}
	var out [3]float64
	for i, v := range linear {
		v = math.Min(1, math.Max(0, v))
		if v <= 0.0031308 {
			out[i] = 12.92 * v
		} else {
			out[i] = 1.055*math.Pow(v, 1/2.4) - 0.055
		}
	}
	return out
}

// GenerateTokens returns the Go source with the tokens of a theme file
// (REQ-STY-14). The keys are the names of the custom properties.
func GenerateTokens(theme []byte) []byte {
	light, dark := ThemeValues(theme)
	var b bytes.Buffer
	b.WriteString("// Code generated by gx. DO NOT EDIT.\n\n")
	b.WriteString("package gxstyles\n\n")
	write := func(name, mode string, tokens map[string]string) {
		fmt.Fprintf(&b, "// %s returns the tokens of app/theme.css for the %s theme as values\n", name, mode)
		b.WriteString("// that an email client reads: hex colours and pixel sizes (REQ-STY-14).\n")
		b.WriteString("// Give it to gx.EmailOptions.\n")
		fmt.Fprintf(&b, "func %s() map[string]string {\n\treturn map[string]string{\n", name)
		names := make([]string, 0, len(tokens))
		width := 0
		for n := range tokens {
			names = append(names, n)
			width = max(width, len(strconv.Quote(n)))
		}
		sort.Strings(names)
		for _, n := range names {
			key := strconv.Quote(n) + ":"
			fmt.Fprintf(&b, "\t\t%-*s %s,\n", width+1, key, strconv.Quote(tokens[n]))
		}
		b.WriteString("\t}\n}\n")
	}
	write("LightTokens", "light", light)
	b.WriteString("\n")
	write("DarkTokens", "dark", dark)
	return b.Bytes()
}
