package carousel

import "github.com/alternayte/gx"

// slide returns one demo slide with a number.
func slide(n string) gx.Node {
	return CarouselItem(CarouselItemProps{Children: gx.El("div", gx.Attrs{{Key: "class", Value: "flex h-40 items-center justify-center rounded-md border border-border bg-card text-4xl font-semibold text-card-foreground"}}, gx.Text(n))})
}

var CarouselFixtures = gx.Fixtures[CarouselProps]{
	"Default":  {Id: "carousel-default", Label: "Numbers", Children: gx.Frag(slide("1"), slide("2"), slide("3"))},
	"Vertical": {Id: "carousel-vertical", Label: "Numbers", Orientation: Vertical, Children: gx.Frag(slide("1"), slide("2"), slide("3"))},
}
