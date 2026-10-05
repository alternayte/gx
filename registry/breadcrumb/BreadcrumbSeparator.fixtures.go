package breadcrumb

import "github.com/alternayte/gx"

var BreadcrumbSeparatorFixtures = gx.Fixtures[BreadcrumbSeparatorProps]{"Default": {}}

// BreadcrumbSeparatorWrap renders the separator inside a list, as a page
// uses it.
func BreadcrumbSeparatorWrap(n gx.Node) gx.Node {
	return BreadcrumbList(BreadcrumbListProps{Children: n})
}
