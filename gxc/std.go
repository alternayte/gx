package gxc

import "strings"

// contains and index wrap the standard library so that gx.js can provide
// the same results without importing anything.
func contains(s, substr string) bool { return strings.Contains(s, substr) }

func index(s, substr string) int { return strings.Index(s, substr) }
