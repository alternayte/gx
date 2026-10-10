// Package email holds the emails of the basket slice. The package has the
// name email, so the compiler reports each construct that an email cannot
// hold (GX6010).
package email

// Item is one line of an order in the order email.
type Item struct {
	Name  string
	Qty   int
	Price string
}
