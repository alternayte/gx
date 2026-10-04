package collapsible

import "github.com/alternayte/gx"

var CollapsibleFixtures = gx.Fixtures[CollapsibleProps]{
	"Open":   {Summary: gx.Text("Show details"), Open: true, Children: gx.Text("Hidden content.")},
	"Closed": {Summary: gx.Text("Show details"), Children: gx.Text("Hidden content.")},
}
