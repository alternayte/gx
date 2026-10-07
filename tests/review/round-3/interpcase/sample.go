// Package interpcase holds Go functions that review round 3 runs two times:
// as compiled code and as interpreted code (REQ-DEV-05). Each function is
// plain Go of the part that the interpreter accepts. The test reads this
// file as the source of the interpreted run.
package interpcase

import "fmt"

// Point is a row of the samples.
type Point struct {
	X, Y int
}

// String is a method with a value receiver.
func (p Point) String() string { return fmt.Sprint("P", p.X, ",", p.Y) }

// Swap exchanges values with a tuple assignment. Go reads every value of
// the right side before it writes the first name of the left side.
func Swap() string {
	a, b := 1, 2
	a, b = b, a
	first, second, third := "x", "y", "z"
	first, second, third = third, first, second
	p, q := Point{1, 2}, Point{3, 4}
	p, q = q, p
	var r Point
	r.X = 5
	r.X, r.Y = 3, r.X
	return fmt.Sprint(a, b, first, second, third, p, q, r)
}

// IndexBeforeWrite assigns to a name and to an element that the name
// indexes. Go reads the index of the left side before it writes a value.
func IndexBeforeWrite() string {
	rows := []string{"a", "b", "c"}
	i := 0
	i, rows[i] = 2, "changed"
	return fmt.Sprint(i, rows)
}

// RangeOnce changes the operand of a range inside the loop. Go reads the
// operand one time: a range over an array reads a copy, and a range over a
// slice keeps the slice and the length of the start.
func RangeOnce() string {
	cells := [3]int{1, 2, 3}
	sum := 0
	for _, v := range cells {
		cells[2] = 10
		sum += v
	}
	rows := []string{"a", "b", "c"}
	seen := ""
	for i, row := range rows {
		if i == 0 {
			rows = []string{"x", "y", "z", "more"}
		}
		seen += row
	}
	return fmt.Sprint(sum, seen, len(rows))
}

// MethodValue takes a method value of a value receiver and then changes the
// variable. Go copies the receiver at the time it makes the method value.
func MethodValue() string {
	p := Point{1, 2}
	label := p.String
	p.X = 50
	return label()
}
