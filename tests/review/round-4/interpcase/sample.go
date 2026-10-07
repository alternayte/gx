// Package interpcase holds Go functions that review round 4 runs two times:
// as compiled code and as interpreted code (REQ-DEV-05). Each function is
// plain Go of the part that the interpreter accepts. The test reads this
// file as the source of the interpreted run.
package interpcase

import (
	"fmt"
	"math"
)

// Sep is an untyped rune constant of the package.
const Sep = '/'

var calls int

// next counts its calls and returns the count.
func next() int {
	calls++
	return calls
}

// ConstDivision divides a constant of a float type by a whole number. The
// type of a constant decides the division, not the form of its value:
// float64(1) / 2 is 0.5.
func ConstDivision() string {
	half := float64(1) / 2
	ratio := float64(7) / float64(2)
	var small float32 = float32(3) / 4
	return fmt.Sprint(half, ratio, small)
}

// NaNOrder compares with a value that is not a number, for example the
// mean of no rows. Every order comparison with NaN is false in Go.
func NaNOrder() string {
	sum, rows := 0.0, 0.0
	mean := sum / rows
	return fmt.Sprint(mean <= 1, mean >= 1, 1 <= mean, 1 >= mean, mean < 1, mean > 1)
}

// NaNMinMax takes min and max with NaN. Go gives NaN when one value is NaN.
func NaNMinMax() string {
	nan := math.NaN()
	one := 1.0
	return fmt.Sprint(min(one, nan), max(one, nan), min(nan, one))
}

// OpAssignOnce has a call in the operand of ++ and of +=. Go evaluates the
// operand of the left side one time.
func OpAssignOnce() string {
	calls = 0
	counts := make([]int, 8)
	counts[next()]++
	counts[next()] += 5
	byKey := map[int]int{}
	byKey[next()] += 2
	return fmt.Sprint(calls, counts, byKey)
}

// RuneConst uses untyped rune constants: 'a' + 1 and a constant of the
// package. The default type of an untyped rune constant is rune.
func RuneConst() string {
	x := 'a' + 1
	var boxed any = x
	_, isRune := boxed.(rune)
	return fmt.Sprint(isRune, any(Sep) == any('/'))
}
