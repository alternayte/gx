package inputgroup

import "github.com/alternayte/gx"

var InputGroupInputFixtures = gx.Fixtures[InputGroupInputProps]{"Placeholder": {Placeholder: "Value", Attrs: name("Value")}}

// InputGroupInputWrap renders the input in a group, which draws its border
// and its focus ring.
func InputGroupInputWrap(n gx.Node) gx.Node {
	return InputGroup(InputGroupProps{Children: n})
}
