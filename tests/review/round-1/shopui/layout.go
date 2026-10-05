// Package shopui holds the layout of one slice for the round-1 review test
// TestREQ_RTE_12_LayoutsWithTheSameFileNameAndLine.
package shopui

import "github.com/alternayte/gx"

// Layout is declared in layout.go at the same line as adminui.Layout.
var Layout = gx.Layout(nil, func(_ struct{}, children gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "id", Value: "shop-layout"}}, children)
})
