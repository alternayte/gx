package docs

import "github.com/alternayte/gx"

var LLMSkipFixtures = gx.Fixtures[LLMSkipProps]{
	"Default": {Children: gx.Text("Only for the page.")},
}
