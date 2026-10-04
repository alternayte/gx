package docs

import "github.com/alternayte/gx"

var AsideFixtures = gx.Fixtures[AsideProps]{
	"Note":    {Kind: Note, Title: "Note", Children: gx.Text("A note.")},
	"Tip":     {Kind: Tip, Children: gx.Text("A tip.")},
	"Caution": {Kind: Caution, Children: gx.Text("Careful.")},
	"Danger":  {Kind: Danger, Title: "Danger", Children: gx.Text("Stop.")},
}
