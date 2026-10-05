package breadcrumb

import "github.com/alternayte/gx"

var BreadcrumbFixtures = gx.Fixtures[BreadcrumbProps]{
	"Trail": {Children: BreadcrumbList(BreadcrumbListProps{Children: gx.Frag(
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbLink(BreadcrumbLinkProps{Href: gx.URL("/"), Children: gx.Text("Home")})}),
		BreadcrumbSeparator(BreadcrumbSeparatorProps{}),
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbLink(BreadcrumbLinkProps{Href: gx.URL("/docs"), Children: gx.Text("Docs")})}),
		BreadcrumbSeparator(BreadcrumbSeparatorProps{}),
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbPage(BreadcrumbPageProps{Children: gx.Text("Breadcrumb")})}),
	)})},
	"Ellipsis": {Children: BreadcrumbList(BreadcrumbListProps{Children: gx.Frag(
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbLink(BreadcrumbLinkProps{Href: gx.URL("/"), Children: gx.Text("Home")})}),
		BreadcrumbSeparator(BreadcrumbSeparatorProps{}),
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbEllipsis(BreadcrumbEllipsisProps{})}),
		BreadcrumbSeparator(BreadcrumbSeparatorProps{}),
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbPage(BreadcrumbPageProps{Children: gx.Text("Breadcrumb")})}),
	)})},
	"CustomSeparator": {Children: BreadcrumbList(BreadcrumbListProps{Children: gx.Frag(
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbLink(BreadcrumbLinkProps{Href: gx.URL("/"), Children: gx.Text("Home")})}),
		BreadcrumbSeparator(BreadcrumbSeparatorProps{Children: gx.Text("/")}),
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbPage(BreadcrumbPageProps{Children: gx.Text("Breadcrumb")})}),
	)})},
}
