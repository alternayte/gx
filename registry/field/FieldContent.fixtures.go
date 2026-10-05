package field

import "github.com/alternayte/gx"

var FieldContentFixtures = gx.Fixtures[FieldContentProps]{
	"TitleAndDescription": {Children: gx.Frag(
		FieldTitle(FieldTitleProps{Children: gx.Text("Product news")}),
		FieldDescription(FieldDescriptionProps{Children: gx.Text("One message each month.")}),
	)},
}
