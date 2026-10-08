package round6_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

// notesPlugin is a plugin with one tool for gx mcp. The tool takes no
// arguments, and its author wrote the JSON Schema that accepts each value.
type notesPlugin struct{ schema string }

func (notesPlugin) Name() string { return "notes" }
func (notesPlugin) API() int     { return gxcli.PluginAPI }
func (p notesPlugin) MCPTools() []gxcli.MCPTool {
	return []gxcli.MCPTool{{
		Name:        "notes_count",
		Description: "Counts the notes of the project.",
		InputSchema: json.RawMessage(p.schema),
		Call:        func(context.Context, json.RawMessage) (string, error) { return "0", nil },
	}}
}

// TestREQ_PLG_05_MCPToolSchemaIsCheckedAtPluginLoad checks the load check of
// the MCPTools hook (REQ-PLG-01: the hook MCPTools; REQ-PLG-05 and D-291:
// "The load checks run before each command", and a plugin that cannot load
// "fails with a clear error"). The check of a tool takes each input schema
// that is JSON. The MCP SDK of gx mcp takes only a schema with the type
// "object" and panics for each other one. The schema {} is valid JSON Schema
// and is what an author writes for a tool with no arguments: the plugin
// loads, each command of the project runs, and gx mcp stops with a Go panic
// and a stack trace in the place of an error that names the plugin.
func TestREQ_PLG_05_MCPToolSchemaIsCheckedAtPluginLoad(t *testing.T) {
	for _, schema := range []string{`{}`, `{"type":"string"}`, `true`} {
		t.Run(schema, func(t *testing.T) {
			// gx mcp talks on stdin and stdout. An empty stdin ends the
			// server at once when it starts.
			null, err := os.Open(os.DevNull)
			if err != nil {
				t.Fatal(err)
			}
			defer null.Close()
			sink, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer sink.Close()
			stdin, stdout, stderr := os.Stdin, os.Stdout, os.Stderr
			os.Stdin, os.Stdout, os.Stderr = null, sink, sink
			var panicked any
			code := -1
			func() {
				defer func() { panicked = recover() }()
				code = gxcli.Main([]string{"mcp", t.TempDir()}, gxcli.WithPlugins(notesPlugin{schema}))
			}()
			// A run with no plugin: the plugins of this test do not stay
			// for a later test of the package.
			gxcli.Main([]string{"help"})
			os.Stdin, os.Stdout, os.Stderr = stdin, stdout, stderr
			if panicked != nil {
				t.Errorf("gx mcp with a plugin tool whose input schema is %s stopped with a panic: %s; want an error at the load of the plugin that names the plugin and the tool (exit code %d)", schema, fmt.Sprint(panicked), code)
			}
		})
	}
}
