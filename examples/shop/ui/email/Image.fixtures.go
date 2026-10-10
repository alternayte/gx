package email

import "github.com/alternayte/gx"

// pixel is a grey image of one pixel, so the gallery needs no file.
const pixel = gx.URL("data:image/gif;base64,R0lGODlhAQABAIAAAMzMzAAAACH5BAAAAAAALAAAAAABAAEAAAICRAEAOw==")

// ImageFixtures are the examples of Image in the dev gallery.
var ImageFixtures = gx.Fixtures[ImageProps]{
	"Default": {Src: pixel, Alt: "The logo of the shop", Width: 120, Height: 40},
}
