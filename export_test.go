package gx

// ElementByID gives the tests of the package the lookup that a form
// re-render uses (REQ-FRM-05).
func ElementByID(n Node, id string) Node { return findElementByID(n, id) }
