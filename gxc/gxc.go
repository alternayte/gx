// Package gxc holds typed helpers that a client expression can call. Each
// helper has the same result in Go and in JavaScript (REQ-ACT-13).
package gxc

// Len returns the number of Unicode code points in s. Use it instead of
// len, because len counts bytes in Go and code units in JavaScript.
func Len(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// At returns the code point at rune index i, or "" when i is out of range.
// Use it instead of s[i], which indexes bytes in Go.
func At(s string, i int) string {
	if i < 0 {
		return ""
	}
	for _, r := range s {
		if i == 0 {
			return string(r)
		}
		i--
	}
	return ""
}

// Contains reports whether s holds substr.
func Contains(s, substr string) bool { return contains(s, substr) }

// Index returns the rune index of substr in s, or -1.
func Index(s, substr string) int {
	at := index(s, substr)
	if at < 0 {
		return -1
	}
	n := 0
	for i := range s {
		if i == at {
			return n
		}
		n++
	}
	return -1
}
