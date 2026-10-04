package breadcrumb

import "github.com/alternayte/gx"

var BreadcrumbItemFixtures = gx.Fixtures[BreadcrumbItemProps]{"Empty": {}}

// BreadcrumbItemWrap renders the item inside a list, as a page uses it.
func BreadcrumbItemWrap(n gx.Node) gx.Node {
	return BreadcrumbList(BreadcrumbListProps{Children: n})
}
