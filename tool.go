package gx

// ToolInfo describes one tool: an action or a form that an agent can call
// (REQ-AI-06). Generated code makes it from the input type. The name comes
// from the type, and the description from the doc comment of the action.
// The JSON Schema comes from the fields and the rules of the input
// (REQ-AI-09).
type ToolInfo struct {
	Name        string
	Description string
	// Schema is the JSON Schema of the arguments.
	Schema string
	// Fields are the top-level arguments, in the order of the struct.
	Fields []ToolField
	// Confirm is true for a tool with gx.Confirm: a client asks the user
	// before an agent runs it. ReadOnly is true for a tool whose action has
	// the method GET. App.Tools sets the two; generated code does not.
	Confirm  bool
	ReadOnly bool
}

// ToolField is one top-level argument of a tool. In names the part of the
// request that the binder reads it from: "path", "query", "form" or
// "signal".
type ToolField struct {
	Name string
	In   string
}

// ToolOption changes how an agent can call a tool.
type ToolOption struct {
	confirm bool
}

// Confirm makes the browser ask the user before an agent runs the tool. Use
// it for an action that the user cannot undo.
var Confirm = ToolOption{confirm: true}

// toolDef is the tool of one action or form.
type toolDef struct {
	info    ToolInfo
	confirm bool
}

// newTool reads the generated description of the input type In.
func newTool[In any](opts []ToolOption) *toolDef {
	var zero In
	described, ok := any(zero).(interface{ GxTool() ToolInfo })
	if !ok {
		panic("gx: the input type of a tool needs a generated GxTool method; run gx generate")
	}
	t := &toolDef{info: described.GxTool()}
	for _, o := range opts {
		if o.confirm {
			t.confirm = true
		}
	}
	return t
}

// Tool registers the action as a tool for an agent (REQ-AI-06). The name
// comes from the input type, the description from the doc comment of the
// variable of the action, and the input schema from the input struct and its
// rules. Only an action with Tool is a tool (SI-07).
func (a *action[In]) Tool(opts ...ToolOption) *action[In] {
	a.tool = newTool[In](opts)
	return a
}

// Tool registers the form as a tool for an agent, as Tool of an action does
// (REQ-AI-06). A call of the tool runs the rules of the form first.
func (f *form[In, P]) Tool(opts ...ToolOption) *form[In, P] {
	described, ok := f.newIn().(interface{ GxTool() ToolInfo })
	if !ok {
		panic("gx: the input type of a tool needs a generated GxTool method; run gx generate")
	}
	f.tool = &toolDef{info: described.GxTool()}
	for _, o := range opts {
		if o.confirm {
			f.tool.confirm = true
		}
	}
	return f
}

// toolDef returns the tool of the form, or nil.
func (f *form[In, P]) toolDef() *toolDef { return f.tool }

// toolDef returns the tool of the action, or nil.
func (a *action[In]) toolDef() *toolDef { return a.tool }
