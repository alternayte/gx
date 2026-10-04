package breadcrumb

import "github.com/alternayte/gx"

var breadcrumbURL = gx.URL("/")
var breadcrumbDocs = gx.URL("/docs")

var BreadcrumbFixtures = gx.Fixtures[BreadcrumbProps]{
	"Trail": {Children: BreadcrumbList(BreadcrumbListProps{Children: gx.Frag(
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbLink(BreadcrumbLinkProps{Href: breadcrumbURL, Children: gx.Text("Home")})}),
		BreadcrumbSeparator(BreadcrumbSeparatorProps{}),
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbLink(BreadcrumbLinkProps{Href: breadcrumbDocs, Children: gx.Text("Docs")})}),
		BreadcrumbSeparator(BreadcrumbSeparatorProps{}),
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbPage(BreadcrumbPageProps{Children: gx.Text("Breadcrumb")})}),
	)})},
	"Ellipsis": {Children: BreadcrumbList(BreadcrumbListProps{Children: gx.Frag(
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbLink(BreadcrumbLinkProps{Href: breadcrumbURL, Children: gx.Text("Home")})}),
		BreadcrumbSeparator(BreadcrumbSeparatorProps{}),
		BreadcrumbItem(BreadcrumbItemProps{Children: BreadcrumbEllipsis(BreadcrumbEllipsisProps{})}),
	)})},
}

var BreadcrumbListFixtures = gx.Fixtures[BreadcrumbListProps]{"Empty": {}}
var BreadcrumbItemFixtures = gx.Fixtures[BreadcrumbItemProps]{"Empty": {}}
var BreadcrumbLinkFixtures = gx.Fixtures[BreadcrumbLinkProps]{"Link": {Href: breadcrumbURL, Children: gx.Text("Home")}}
var BreadcrumbPageFixtures = gx.Fixtures[BreadcrumbPageProps]{"Page": {Children: gx.Text("Page")}}
var BreadcrumbSeparatorFixtures = gx.Fixtures[BreadcrumbSeparatorProps]{"Default": {}}
var BreadcrumbEllipsisFixtures = gx.Fixtures[BreadcrumbEllipsisProps]{"Default": {}}
