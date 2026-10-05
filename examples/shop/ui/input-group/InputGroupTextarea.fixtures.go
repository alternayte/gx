package inputgroup

import "github.com/alternayte/gx"

var InputGroupTextareaFixtures = gx.Fixtures[InputGroupTextareaProps]{
	"Placeholder": {Placeholder: "Ask a question.", Attrs: name("Question")},
}

// InputGroupTextareaWrap renders the textarea in a group, which draws its
// border and its focus ring.
func InputGroupTextareaWrap(n gx.Node) gx.Node {
	return InputGroup(InputGroupProps{Children: n})
}
