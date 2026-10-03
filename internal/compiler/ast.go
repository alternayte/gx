package compiler

// Pos is a 1-based position in a .gx file.
type Pos struct {
	Line int
	Col  int
}

// File is the parsed form of one .gx file or fragment.
type File struct {
	File       string
	Package    string
	Imports    []Import
	HasProps   bool
	Props      []Field
	HasSignals bool
	Signals    []Field
	Body       []Node
}

// Import is one import spec, as written, without the import keyword.
type Import struct {
	Raw string
}

// Field is one props or signals field.
type Field struct {
	At         Pos
	Name       string
	Type       string
	Default    string
	HasDefault bool
}

// Node is a markup node.
type Node interface {
	Position() Pos
	node()
}

// Text is literal markup text.
type Text struct {
	At   Pos
	Data string
}

// Expr is {expr} in text or an attribute value.
type Expr struct {
	At     Pos
	DataAt Pos
	Data   string
}

// Comment is a {/* ... */} comment. It never renders.
type Comment struct {
	At   Pos
	Data string
}

// HTMLComment is a <!-- ... --> comment. It renders.
type HTMLComment struct {
	At   Pos
	Data string
}

// Let is a name := expr statement line.
type Let struct {
	At   Pos
	Name string
	Expr string
}

// Element is an HTML element, a component tag or a slot.
type Element struct {
	At        Pos
	Name      string
	Attrs     []Attr
	Children  []Node
	RawText   string
	HasRaw    bool
	SelfClose bool
}

// Control is an if, for or switch block.
type Control struct {
	At     Pos
	Kind   string
	Header string
	Body   []Node
	Else   []Node
	Cases  []Case
}

// Case is one switch clause.
type Case struct {
	At        Pos
	Header    string
	IsDefault bool
	Body      []Node
}

// Attr is one attribute of an element.
type Attr struct {
	At      Pos
	Kind    AttrKind
	Name    string
	Value   string
	ValueAt Pos
}

// AttrKind tells how an attribute value is written.
type AttrKind int

const (
	AttrBool AttrKind = iota
	AttrString
	AttrExpr
	AttrSpread
	AttrFragment
)

func (a *Attr) Position() Pos        { return a.At }
func (n *Text) Position() Pos        { return n.At }
func (n *Expr) Position() Pos        { return n.At }
func (n *Comment) Position() Pos     { return n.At }
func (n *HTMLComment) Position() Pos { return n.At }
func (n *Let) Position() Pos         { return n.At }
func (n *Element) Position() Pos     { return n.At }
func (n *Control) Position() Pos     { return n.At }

func (n *Text) node()        {}
func (n *Expr) node()        {}
func (n *Comment) node()     {}
func (n *HTMLComment) node() {}
func (n *Let) node()         {}
func (n *Element) node()     {}
func (n *Control) node()     {}

// voidElements are HTML elements that never have children.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// rawElements hold text that is not markup.
var rawElements = map[string]bool{
	"script": true, "style": true,
}

// VisibleNodes returns the nodes a renderer emits: all nodes except
// {/* ... */} comments (REQ-AUT-16).
func VisibleNodes(ns []Node) []Node {
	out := make([]Node, 0, len(ns))
	for _, n := range ns {
		if _, ok := n.(*Comment); ok {
			continue
		}
		out = append(out, n)
	}
	return out
}
