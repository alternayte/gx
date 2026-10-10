// Package home is a fixture slice for the review tests. Its Shell component
// has props with route types of two slices. Each slice has a package with
// the name route (DR-01).
package home

import (
	cartroute "github.com/alternayte/gx/tests/review/round-8/fix/cart/route"
	productsroute "github.com/alternayte/gx/tests/review/round-8/fix/products/route"
)

// ShellProps are the props of the Shell component.
type ShellProps struct {
	Add  cartroute.Add
	Show productsroute.Show
}
