package gx

import (
	"strconv"
	"strings"
)

// errorHeader marks the answer of an action whose handler gave an error. The
// answer has status 200, so that the toast reaches the browser (REQ-ACT-10);
// the runtime reads the header to put optimistic signals back (REQ-ACT-18).
const errorHeader = "Gx-Error"

// KeepSignal returns the text that reads one signal of a component instance
// into an object with the shape of the signals of the page. Generated code
// gives it to Keep for each signal that an optimistic directive writes
// (REQ-ACT-18).
func KeepSignal(base string, key Key, name string) string {
	parts := scopePath(base, key)
	var b strings.Builder
	for _, part := range parts {
		b.WriteByte('{')
		b.WriteString(strconv.Quote(part))
		b.WriteByte(':')
	}
	b.WriteByte('{')
	b.WriteString(strconv.Quote(name))
	b.WriteByte(':')
	b.WriteString(SignalPath(base, key, name))
	b.WriteString(strings.Repeat("}", len(parts)+1))
	return b.String()
}

// Keep returns the statement that gives the values of signals to the Gx
// runtime before an optimistic directive changes them. The runtime puts the
// values back when the action of the element fails (REQ-ACT-18).
func Keep(signals ...string) string {
	return "window.__gx&&window.__gx.keep([" + strings.Join(signals, ",") + "]);"
}
