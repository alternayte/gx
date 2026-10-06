package carousel

import "github.com/alternayte/gx"

var CarouselItemFixtures = gx.Fixtures[CarouselItemProps]{
	"Default": {Children: gx.El("div", gx.Attrs{{Key: "class", Value: "flex h-40 items-center justify-center rounded-md border border-border bg-card text-4xl font-semibold text-card-foreground"}}, gx.Text("1"))},
}
