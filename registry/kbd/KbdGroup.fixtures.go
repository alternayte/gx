package kbd

import "github.com/alternayte/gx"

var KbdGroupFixtures = gx.Fixtures[KbdGroupProps]{
	"Shortcut": {Children: gx.Frag(
		Kbd(KbdProps{Children: gx.Text("Ctrl")}),
		gx.El("span", nil, gx.Text("+")),
		Kbd(KbdProps{Children: gx.Text("K")}),
	)},
}
