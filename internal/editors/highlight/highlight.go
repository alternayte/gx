// Package highlight holds the highlighting snapshot tests of the editors
// (REQ-DEV-09). The tests need Bun, npx and a C compiler, so they are in a
// package of their own: the platform jobs of CI leave it out and the gate
// runs it.
package highlight
