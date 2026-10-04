package breadcrumb

import "github.com/alternayte/gx"

var BreadcrumbLinkFixtures = gx.Fixtures[BreadcrumbLinkProps]{"Link": {Href: gx.URL("/"), Children: gx.Text("Home")}}
