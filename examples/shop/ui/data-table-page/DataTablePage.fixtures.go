package datatablepage

import "github.com/alternayte/gx"

var DataTablePageFixtures = gx.Fixtures[DataTablePageProps]{
	"Default": {
		Title:       "Invoices",
		Description: "Every invoice for this workspace.",
		Filter:      gx.Text("Search"),
		Table:       gx.Text("Table slot"),
		Pagination:  gx.Text("Pagination slot"),
	},
}
