package gx

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Cx merges class strings with the semantics of tailwind-merge for Tailwind
// v4 (REQ-STY-04). A later class removes an earlier class of the same or a
// conflicting class group, a modifier scopes the conflict, and the classes
// that stay keep their order. A class that Tailwind does not know stays.
//
// The class groups come from the tailwind-merge default configuration: the
// table in cxtable.go is generated from it, and the functions below are a
// port of its merge, parse and validator code.
func Cx(parts ...string) string {
	var tokenBuf [24]string
	tokens := tokenBuf[:0]
	size := 0
	for _, part := range parts {
		for i := 0; i < len(part); {
			for i < len(part) && isClassSpace(part[i]) {
				i++
			}
			if i >= len(part) {
				break
			}
			start := i
			for i < len(part) && !isClassSpace(part[i]) {
				i++
			}
			tokens = append(tokens, part[start:i])
			size += i - start + 1
		}
	}
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) == 1 {
		return tokens[0]
	}

	// The last class wins, so the walk goes from the end. seen holds the
	// class groups a later class already took, each under its modifiers.
	var seenBuf [64]cxSeen
	seen := seenBuf[:0]
	var arbitraryBuf [4]cxArbitrary
	arbitrary := arbitraryBuf[:0]
	// scopes holds each distinct modifier scope of the call once, so a seen
	// entry is four bytes and a comparison is one word.
	var scopeBuf [8]cxScope
	scopes := scopeBuf[:0]
	var dropBuf [24]bool
	drop := dropBuf[:0]
	if len(tokens) > len(dropBuf) {
		drop = make([]bool, len(tokens))
	} else {
		drop = dropBuf[:len(tokens)]
	}
	dropped := false
	for i := len(tokens) - 1; i >= 0; i-- {
		id, postfix, ok := classID(tokens[i])
		if !ok {
			continue
		}
		scope := -1
		for j := range scopes {
			if scopes[j].important == id.important && scopes[j].modifiers == id.modifiers {
				scope = j
				break
			}
		}
		if scope < 0 {
			scope = len(scopes)
			scopes = append(scopes, cxScope{id.modifiers, id.important})
		}
		taken := false
		if id.arbitrary != "" {
			for _, a := range arbitrary {
				if a.scope == scope && a.property == id.arbitrary {
					taken = true
					break
				}
			}
			if !taken {
				arbitrary = append(arbitrary, cxArbitrary{scope, id.arbitrary})
			}
		} else {
			key := cxSeen{uint16(scope), id.group}
			for _, s := range seen {
				if s == key {
					taken = true
					break
				}
			}
			if !taken {
				seen = append(seen, key)
				for _, g := range twConflicts[id.group] {
					seen = append(seen, cxSeen{uint16(scope), g})
				}
				if postfix {
					for _, g := range twModifierConflicts[id.group] {
						seen = append(seen, cxSeen{uint16(scope), g})
					}
				}
			}
		}
		if taken {
			drop[i], dropped = true, true
			size -= len(tokens[i]) + 1
		}
	}
	if !dropped && len(parts) == 1 && len(parts[0]) == size-1 && strings.IndexAny(parts[0], "\t\n\r\f\v") < 0 {
		// One argument with single spaces and no conflict is the answer.
		return parts[0]
	}
	var b strings.Builder
	b.Grow(size)
	for i, token := range tokens {
		if drop[i] {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(token)
	}
	return b.String()
}

// cxID names what one class sets: its class group under its modifiers. Two
// classes with the same cxID conflict.
type cxID struct {
	// modifiers is the variant part in canonical order, for example
	// "hover:focus".
	modifiers string
	important bool
	group     uint16
	// arbitrary is the property of an arbitrary property class, such as
	// "mask-type" for [mask-type:alpha]. Such a class has no group.
	arbitrary string
}

// cxScope is one modifier scope of a call: the canonical modifiers and the
// important mark.
type cxScope struct {
	modifiers string
	important bool
}

// cxSeen is a class group that a later class took in one scope.
type cxSeen struct {
	scope uint16
	group uint16
}

// cxArbitrary is an arbitrary property that a later class set in one scope.
type cxArbitrary struct {
	scope    int
	property string
}

// twNode is one node of the class map. A class is split at "-" and each
// part selects a child; the validators take the rest of the class.
type twNode struct {
	group            uint16
	kidStart, kidEnd uint16
	valStart, valEnd uint16
}

type twKid struct {
	part string
	node uint16
}

type twVal struct {
	validator uint8
	group     uint16
}

// isClassSpace reports whether c separates two classes.
func isClassSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// classID parses one class and finds its class group. ok is false for a
// class that is not a Tailwind class. postfix reports a postfix modifier,
// as the "/7" of text-lg/7.
func classID(class string) (id cxID, postfix bool, ok bool) {
	// Split at the ":" and find the last "/" outside brackets and
	// parentheses, as Tailwind does.
	brackets, parens := 0, 0
	modifierStart, modifierCount := 0, 0
	slash := -1
	for i := 0; i < len(class); i++ {
		c := class[i]
		if brackets == 0 && parens == 0 {
			if c == ':' {
				modifierStart = i + 1
				modifierCount++
				continue
			}
			if c == '/' {
				slash = i
				continue
			}
		}
		switch c {
		case '[':
			brackets++
		case ']':
			brackets--
		case '(':
			parens++
		case ')':
			parens--
		}
	}
	base := class[modifierStart:]
	// The position of the "/" counts from the start of the base class
	// with its important mark.
	postfixAt := -1
	if slash > 0 && slash > modifierStart {
		postfixAt = slash - modifierStart
	}
	switch {
	case strings.HasSuffix(base, "!"):
		base = base[:len(base)-1]
		id.important = true
	case strings.HasPrefix(base, "!"):
		// The Tailwind v3 position of the important mark.
		base = base[1:]
		id.important = true
	}

	postfix = postfixAt >= 0
	var group uint16
	var arbitrary string
	if postfix {
		end := postfixAt
		if end > len(base) {
			end = len(base)
		}
		group, arbitrary = classGroup(base[:end])
		if group != 0 && twPostfixLookup[group] {
			if whole, _ := classGroup(base); whole != 0 && whole != group {
				group, postfix = whole, false
			}
		}
	} else {
		group, arbitrary = classGroup(base)
	}
	if group == 0 && arbitrary == "" {
		if !postfix {
			return cxID{}, false, false
		}
		group, arbitrary = classGroup(base)
		if group == 0 && arbitrary == "" {
			return cxID{}, false, false
		}
		postfix = false
	}
	id.group, id.arbitrary = group, arbitrary
	if modifierCount > 0 {
		id.modifiers = canonicalModifiers(class[:modifierStart-1], modifierCount)
	}
	return id, postfix, true
}

// canonicalModifiers returns the modifiers of a class in the order that
// makes two equal sets compare equal: a run of plain modifiers is sorted,
// and an arbitrary or order-sensitive modifier keeps its place.
func canonicalModifiers(modifiers string, count int) string {
	if count == 1 {
		return modifiers
	}
	var buf [8]string
	list := buf[:0]
	brackets, parens, start := 0, 0, 0
	for i := 0; i < len(modifiers); i++ {
		switch c := modifiers[i]; {
		case c == ':' && brackets == 0 && parens == 0:
			list = append(list, modifiers[start:i])
			start = i + 1
		case c == '[':
			brackets++
		case c == ']':
			brackets--
		case c == '(':
			parens++
		case c == ')':
			parens--
		}
	}
	list = append(list, modifiers[start:])
	sorted := true
	segment := 0
	for i := 0; i <= len(list); i++ {
		if i < len(list) && !fixedModifier(list[i]) {
			continue
		}
		run := list[segment:i]
		if !sort.StringsAreSorted(run) {
			sort.Strings(run)
			sorted = false
		}
		segment = i + 1
	}
	if sorted {
		return modifiers
	}
	return strings.Join(list, ":")
}

// fixedModifier reports whether a modifier keeps its position.
func fixedModifier(modifier string) bool {
	return (len(modifier) > 0 && modifier[0] == '[') || twOrderSensitive[modifier]
}

// classGroup returns the class group of a class with no modifier. An
// arbitrary property such as [color:red] has no group; its property comes
// back instead.
func classGroup(class string) (group uint16, arbitrary string) {
	if strings.HasPrefix(class, "[") && strings.HasSuffix(class, "]") && len(class) >= 2 {
		content := class[1 : len(class)-1]
		colon := strings.IndexByte(content, ':')
		if colon <= 0 {
			return 0, ""
		}
		return 0, content[:colon]
	}
	start := 0
	if len(class) > 1 && class[0] == '-' {
		// A negative value: the class map has no part for the sign.
		start = 1
	}
	return twLookup(class, start, 0), ""
}

// twLookup walks the class map from node with the parts of class that
// start at offset start. An offset past the end means that no part is
// left.
func twLookup(class string, start int, node uint16) uint16 {
	n := &twNodes[node]
	if start > len(class) {
		return n.group
	}
	end := strings.IndexByte(class[start:], '-')
	if end < 0 {
		end = len(class)
	} else {
		end += start
	}
	if child, ok := twChild(n, class[start:end]); ok {
		if group := twLookup(class, end+1, child); group != 0 {
			return group
		}
	}
	if n.valStart == n.valEnd {
		return 0
	}
	rest := class[start:]
	for _, v := range twVals[n.valStart:n.valEnd] {
		if twValidate(v.validator, rest) {
			return v.group
		}
	}
	return 0
}

// twChild finds the child of a node for one class part.
func twChild(n *twNode, part string) (uint16, bool) {
	lo, hi := int(n.kidStart), int(n.kidEnd)
	for lo < hi {
		mid := (lo + hi) / 2
		switch k := &twKids[mid]; {
		case k.part == part:
			return k.node, true
		case k.part < part:
			lo = mid + 1
		default:
			hi = mid
		}
	}
	return 0, false
}

// twValidate runs one validator of tailwind-merge on the rest of a class.
func twValidate(validator uint8, value string) bool {
	switch validator {
	case twIsAny:
		return true
	case twIsAnyNonArbitrary:
		return !isArbitraryValue(value) && !isArbitraryVariable(value)
	case twIsFraction:
		return isFraction(value)
	case twIsNumber:
		return isNumber(value)
	case twIsInteger:
		return isInteger(value)
	case twIsPercent:
		return strings.HasSuffix(value, "%") && isNumber(value[:len(value)-1])
	case twIsTshirtSize:
		return isTshirtSize(value)
	case twIsNamedContainerQuery:
		return isNamedContainerQuery(value)
	case twIsArbitraryValue:
		return isArbitraryValue(value)
	case twIsArbitraryVariable:
		return isArbitraryVariable(value)
	case twIsArbitrarySize:
		return arbitraryValue(value, labelSize, never)
	case twIsArbitraryLength:
		return arbitraryValue(value, labelLength, isLengthOnly)
	case twIsArbitraryNumber:
		return arbitraryValue(value, labelNumber, isNumber)
	case twIsArbitraryWeight:
		return arbitraryValue(value, labelWeight, always)
	case twIsArbitraryFamilyName:
		return arbitraryValue(value, labelFamilyName, never)
	case twIsArbitraryPosition:
		return arbitraryValue(value, labelPosition, never)
	case twIsArbitraryImage:
		return arbitraryValue(value, labelImage, imageRe.MatchString)
	case twIsArbitraryShadow:
		return arbitraryValue(value, labelShadow, shadowRe.MatchString)
	case twIsArbitraryVariableLength:
		return arbitraryVariable(value, labelLength, false)
	case twIsArbitraryVariableFamilyName:
		return arbitraryVariable(value, labelFamilyName, false)
	case twIsArbitraryVariablePosition:
		return arbitraryVariable(value, labelPosition, false)
	case twIsArbitraryVariableSize:
		return arbitraryVariable(value, labelSize, false)
	case twIsArbitraryVariableImage:
		return arbitraryVariable(value, labelImage, false)
	case twIsArbitraryVariableShadow:
		return arbitraryVariable(value, labelShadow, true)
	case twIsArbitraryVariableWeight:
		return arbitraryVariable(value, labelWeight, true)
	}
	return false
}

// The patterns of tailwind-merge that are too irregular for a hand-written
// scan. They run only on an arbitrary value in brackets.
var (
	lengthUnitRe    = regexp.MustCompile(`\d+(%|px|r?em|[sdl]?v([hwib]|min|max)|pt|pc|in|cm|mm|cap|ch|ex|r?lh|cq(w|h|i|b|min|max))|\b(calc|min|max|clamp)\(.+\)|^0$`)
	colorFunctionRe = regexp.MustCompile(`^(rgba?|hsla?|hwb|(ok)?(lab|lch)|color-mix|color|light-dark)\(.+\)$`)
	shadowRe        = regexp.MustCompile(`^(inset_)?-?((\d+)?\.?(\d+)[a-z]+|0)_-?((\d+)?\.?(\d+)[a-z]+|0)`)
	imageRe         = regexp.MustCompile(`^(url|image|image-set|cross-fade|element|(repeating-)?(linear|radial|conic)-gradient)\(.+\)$`)
)

func always(string) bool { return true }
func never(string) bool  { return false }

// isLengthOnly reports a length. A colour function can hold a percentage,
// which is not a length.
func isLengthOnly(value string) bool {
	return lengthUnitRe.MatchString(value) && !colorFunctionRe.MatchString(value)
}

func labelPosition(l string) bool   { return l == "position" || l == "percentage" }
func labelImage(l string) bool      { return l == "image" || l == "url" }
func labelSize(l string) bool       { return l == "length" || l == "size" || l == "bg-size" }
func labelLength(l string) bool     { return l == "length" }
func labelNumber(l string) bool     { return l == "number" }
func labelFamilyName(l string) bool { return l == "family-name" }
func labelWeight(l string) bool     { return l == "number" || l == "weight" }
func labelShadow(l string) bool     { return l == "shadow" }

// splitArbitrary splits the inside of [label:value] or (label:value). The
// label is optional: it is a word that can hold hyphens, before the first
// colon, and the value after it is not empty.
func splitArbitrary(value string, open, close byte) (label, rest string, ok bool) {
	if len(value) < 3 || value[0] != open || value[len(value)-1] != close {
		return "", "", false
	}
	inner := value[1 : len(value)-1]
	if strings.ContainsAny(inner, "\n\r") {
		return "", "", false
	}
	if colon := strings.IndexByte(inner, ':'); colon > 0 && colon < len(inner)-1 && isLabel(inner[:colon]) {
		return inner[:colon], inner[colon+1:], true
	}
	return "", inner, true
}

// isLabel reports a word character followed by word characters and
// hyphens.
func isLabel(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		word := c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !word && !(c == '-' && i > 0) {
			return false
		}
	}
	return len(s) > 0
}

func isArbitraryValue(value string) bool {
	_, _, ok := splitArbitrary(value, '[', ']')
	return ok
}

func isArbitraryVariable(value string) bool {
	_, _, ok := splitArbitrary(value, '(', ')')
	return ok
}

// arbitraryValue reports an arbitrary value whose label passes testLabel
// or, with no label, whose value passes testValue.
func arbitraryValue(value string, testLabel, testValue func(string) bool) bool {
	label, rest, ok := splitArbitrary(value, '[', ']')
	if !ok {
		return false
	}
	if label != "" {
		return testLabel(label)
	}
	return testValue(rest)
}

// arbitraryVariable reports an arbitrary variable whose label passes
// testLabel. noLabel is the answer for a variable with no label.
func arbitraryVariable(value string, testLabel func(string) bool, noLabel bool) bool {
	label, _, ok := splitArbitrary(value, '(', ')')
	if !ok {
		return false
	}
	if label != "" {
		return testLabel(label)
	}
	return noLabel
}

// digitsEnd returns the index after the digits that start at i.
func digitsEnd(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

// decimalEnd returns the index after digits with an optional fraction that
// start at i, or -1 when no digit is there.
func decimalEnd(s string, i int) int {
	end := digitsEnd(s, i)
	if end == i {
		return -1
	}
	if end < len(s) && s[end] == '.' {
		if frac := digitsEnd(s, end+1); frac > end+1 {
			return frac
		}
	}
	return end
}

// isFraction reports a value such as 1/2 or 1.5/3.
func isFraction(value string) bool {
	end := decimalEnd(value, 0)
	if end < 0 || end >= len(value) || value[end] != '/' {
		return false
	}
	return decimalEnd(value, end+1) == len(value)
}

// isTshirtSize reports a size such as sm, xl or 2xl.
func isTshirtSize(value string) bool {
	if len(value) < 2 {
		return false
	}
	switch value[len(value)-2:] {
	case "xs", "sm", "md", "lg", "xl":
	default:
		return false
	}
	prefix := value[:len(value)-2]
	return prefix == "" || decimalEnd(prefix, 0) == len(prefix)
}

// isNamedContainerQuery reports @container/name and its size and normal
// forms.
func isNamedContainerQuery(value string) bool {
	if !strings.HasPrefix(value, "@container") {
		return false
	}
	rest := value[len("@container"):]
	switch {
	case strings.HasPrefix(rest, "/"):
		return len(rest) > 1
	case strings.HasPrefix(rest, "-size/"):
		return len(rest) > len("-size/")
	case strings.HasPrefix(rest, "-normal/"):
		return len(rest) > len("-normal/")
	}
	return false
}

// jsNumber reads a string as the JavaScript Number function does. ok is
// false for NaN.
func jsNumber(s string) (value float64, ok bool) {
	if s == "" {
		return 0, false
	}
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			for i := 2; i < len(s); i++ {
				c := s[i]
				digit := 99
				switch {
				case c >= '0' && c <= '9':
					digit = int(c - '0')
				case c >= 'a' && c <= 'f':
					digit = int(c-'a') + 10
				case c >= 'A' && c <= 'F':
					digit = int(c-'A') + 10
				}
				if digit >= base {
					return 0, false
				}
				value = value*float64(base) + float64(digit)
			}
			return value, true
		}
	}
	i := 0
	if s[0] == '+' || s[0] == '-' {
		i = 1
	}
	if s[i:] == "Infinity" {
		return math.Inf(1), true
	}
	whole := digitsEnd(s, i)
	end := whole
	fraction := 0
	if end < len(s) && s[end] == '.' {
		end = digitsEnd(s, end+1)
		fraction = end - whole - 1
	}
	if whole == i && fraction == 0 {
		return 0, false
	}
	if end < len(s) && (s[end] == 'e' || s[end] == 'E') {
		exp := end + 1
		if exp < len(s) && (s[exp] == '+' || s[exp] == '-') {
			exp++
		}
		expEnd := digitsEnd(s, exp)
		if expEnd == exp {
			return 0, false
		}
		end = expEnd
	}
	if end != len(s) {
		return 0, false
	}
	value, err := strconv.ParseFloat(s, 64)
	if err != nil && !math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

// isNumber reports a value that JavaScript reads as a number.
func isNumber(value string) bool {
	_, ok := jsNumber(value)
	return ok
}

// isInteger reports a value that JavaScript reads as a whole number.
func isInteger(value string) bool {
	n, ok := jsNumber(value)
	return ok && !math.IsInf(n, 0) && n == math.Trunc(n)
}
