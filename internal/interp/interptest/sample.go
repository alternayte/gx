// Package interptest holds Go functions that the interpreter tests run two
// times: as compiled code and as interpreted code. Each function takes no
// argument and returns a value that the test compares.
package interptest

import (
	"fmt"
	"strconv"
	"strings"
)

// Item is a row of the samples.
type Item struct {
	Name  string
	Price int
	Tags  []string
	inner int
}

// Label is an exported method with a value receiver.
func (it Item) Label() string { return it.Name + ":" + strconv.Itoa(it.Price) }

// Raise is an exported method with a pointer receiver.
func (it *Item) Raise(by int) { it.Price += by }

// secret is an unexported method; reflection cannot call it.
func (it Item) secret() string { return "s" + strconv.Itoa(it.inner) }

// bump is an unexported method with a pointer receiver.
func (it *Item) bump() { it.inner++ }

// Kind is a named string type with constants.
type Kind string

const (
	Small Kind = "small"
	Large Kind = "large"
)

// Limit is an untyped constant.
const Limit = 3

// Scale is a typed constant.
const Scale float64 = 1.5

// Counter is a package variable that a sample changes.
var Counter = 10

// Render is a named function type, as a slot is.
type Render func(Item) string

// Builder collects text, as the node builder of a template does.
type Builder struct {
	parts []string
}

func (b *Builder) Add(parts ...string) { b.parts = append(b.parts, parts...) }
func (b *Builder) String() string      { return strings.Join(b.parts, "|") }

// Wrapped reaches the fields of an Item through an embedded pointer.
type Wrapped struct {
	*Item
	Count int
}

// Shape is an interface of the samples.
type Shape interface{ Area() int }

// Square is a Shape.
type Square struct{ Side int }

func (s Square) Area() int { return s.Side * s.Side }

func join(sep string, parts ...string) string { return strings.Join(parts, sep) }

func pair(n int) (int, string) { return n * 2, strconv.Itoa(n) }

func apply(r Render, it Item) string { return r(it) }

func items() []Item {
	return []Item{{Name: "a", Price: 1, Tags: []string{"x"}}, {Name: "b", Price: 20}, {Name: "c", Price: 300, Tags: []string{"y", "z"}}}
}

func Arithmetic() string {
	a, b := 7, 2
	var f float64 = 7
	x := uint8(250)
	x += 10
	s := "n"
	s += strconv.Itoa(a/b) + strconv.Itoa(a%b) + strconv.Itoa(a<<3|1) + strconv.Itoa(-a) + strconv.Itoa(a&^b)
	return fmt.Sprint(s, f/2, x, 1<<10, Limit*2, Scale*2, a > b && b >= 2 || false, !(a == b), 'a', 3.0/2, 7/2)
}

func Strings() string {
	name := "héllo"
	out := name + " " + strings.ToUpper(name[:1]) + fmt.Sprint(len(name), name[1], name[2:4] == "\xa9l", name < "z")
	for i, r := range "aé" {
		out += fmt.Sprint(i, r, string(r))
	}
	return out + string(rune(65)) + string(Small) + strconv.Quote(`q"`)
}

func Control() string {
	var b Builder
	for i := 0; i < 5; i++ {
		if i == 1 {
			continue
		}
		if i == 4 {
			break
		}
		b.Add(strconv.Itoa(i))
	}
	n := 0
	for n < 3 {
		n++
	}
	for range 2 {
		n += 10
	}
	for i := range 3 {
		n += i
	}
	if v, s := pair(n); v > 10 {
		b.Add(s, strconv.Itoa(v))
	} else if v == 0 {
		b.Add("zero")
	} else {
		b.Add("small")
	}
	switch k := Kind("large"); k {
	case Small:
		b.Add("S")
	case Large, "huge":
		b.Add("L")
	default:
		b.Add("?")
	}
	switch {
	case n > 100:
		b.Add("big")
	case n > 20:
		b.Add("mid")
	}
	switch n {
	case 1:
		b.Add("one")
	default:
		b.Add("other")
	}
	return b.String()
}

func Ranges() string {
	var b Builder
	total := 0
	for i, it := range items() {
		total += it.Price * (i + 1)
		for _, tag := range it.Tags {
			b.Add(tag)
		}
	}
	m := map[string]int{"a": 1, "b": 2}
	sum := 0
	for k, v := range m {
		sum += v + len(k)
	}
	for k := range m {
		sum += len(k)
	}
	arr := [3]int{1, 2, 3}
	for _, v := range arr {
		sum += v
	}
	return b.String() + fmt.Sprint(total, sum, len(m), m["a"], m["zz"])
}

func Literals() string {
	it := Item{Name: "lit", Price: 4, Tags: []string{"t"}}
	p := &Item{Name: "ptr"}
	p.Raise(5)
	p.Price++
	nested := map[string][]Item{"k": {{Name: "n1"}, {Name: "n2", Price: 2}}}
	pts := []*Item{{Name: "p1"}, p}
	grid := [][]int{{1, 2}, {3}}
	anon := struct {
		A int
		B string
	}{1, "b"}
	var zero Item
	sparse := []string{2: "two", 0: "zero"}
	return fmt.Sprint(it.Label(), p.Label(), len(nested["k"]), nested["k"][1].Price, pts[1].Name, grid[0][1]+grid[1][0], anon.A, anon.B, zero.Name == "", len(sparse), sparse[2], it.Tags[0])
}

func Methods() string {
	it := Item{Name: "m", Price: 1}
	it.Raise(2)
	it.bump()
	it.bump()
	ptr := &it
	ptr.bump()
	var sh Shape = Square{Side: 3}
	label := it.Label
	return fmt.Sprint(it.Label(), it.secret(), ptr.secret(), it.inner, sh.Area(), label(), Square{2}.Area())
}

func Closures() string {
	var fns []func() string
	for i, it := range items() {
		fns = append(fns, func() string { return strconv.Itoa(i) + it.Name })
	}
	out := ""
	for _, fn := range fns {
		out += fn()
	}
	count := 0
	inc := func(by int) int {
		count += by
		return count
	}
	inc(2)
	inc(3)
	row := func(it Item) string { return "<" + it.Name + ">" }
	out += apply(row, Item{Name: "r"}) + apply(func(it Item) string { return it.Label() }, Item{Name: "s", Price: 9})
	var r Render = row
	for j := 0; j < 2; j++ {
		fns = append(fns, func() string { return strconv.Itoa(j) })
	}
	return out + strconv.Itoa(count) + r(Item{Name: "t"}) + fns[3]() + fns[4]()
}

func Calls() string {
	parts := []string{"x", "y"}
	n, s := pair(4)
	a, ok := map[string]int{"k": 1}["k"]
	_, missing := map[string]int{}["k"]
	var any1 any = "text"
	str, isStr := any1.(string)
	_, isInt := any1.(int)
	var sh Shape = Square{4}
	sq := sh.(Square)
	return join("-", "a", "b") + join("+") + join(",", parts...) + fmt.Sprint(n, s, a, ok, missing, str, isStr, isInt, sq.Side, strings.Repeat("ab", 2), min(3, 1, 2), max(2.5, 1), len(parts), cap(parts[:1]) >= 1)
}

func Variables() string {
	Counter += 5
	saved := Counter
	Counter = 10
	var list []int
	list = append(list, 1, 2)
	list = append(list, []int{3, 4}...)
	list[0] = 9
	m := make(map[string][]string)
	m["a"] = append(m["a"], "v")
	delete(m, "zz")
	it := Item{}
	it.Tags = make([]string, 2, 4)
	it.Tags[1] = "last"
	p := &it
	p.Name = "viaptr"
	q := new(int)
	*q = 7
	var iface any
	var nilSlice []string
	var np *Item
	return fmt.Sprint(saved, Counter, list, m, it.Name, it.Tags, *q, iface == nil, nilSlice == nil, np == nil, p != nil, len(nilSlice))
}

func Conversions() string {
	var k Kind = "small"
	n := 300
	f := 2.9
	var i8 int8 = int8(n)
	var b []byte = []byte("hi")
	return fmt.Sprint(string(k), Kind("x") == k, k == Small, float64(n)/7, int(f), i8, uint16(n)*300, string(b), []rune("é")[0], strconv.Itoa(int(int64(n))), Scale*float64(n), any(n) == any(300))
}

// Assignments gives the order in which an assignment reads its operands:
// each call adds its name to the log. Go reads the operands of the left
// side, then the right side, both from left to right, and then stores from
// left to right.
func Assignments() string {
	var log []string
	n := func(name string, v int) int {
		log = append(log, name)
		return v
	}
	xs := []int{0, 0, 0, 0}
	list := func(name string) []int {
		log = append(log, name)
		return xs
	}
	m := map[string]int{}
	table := func(name string) map[string]int {
		log = append(log, name)
		return m
	}
	key := func(name string) string {
		log = append(log, name)
		return name
	}
	a, b := Item{Name: "a"}, Item{Name: "b"}
	item := func(name string) *Item {
		log = append(log, name)
		if name < "b" || name == "i" {
			return &a
		}
		return &b
	}
	p, q := 0, 0
	ptr := func(name string) *int {
		log = append(log, name)
		if name < "b" || name == "m" {
			return &p
		}
		return &q
	}
	step := func(title string) {
		log = append(log, "/"+title+":"+fmt.Sprint(xs, len(m), m["k1"], m["k2"], m["k3"], a.Price, b.Price, p, q)+"\n")
	}

	// Slice elements.
	xs[n("a", 0)], xs[n("b", 1)] = n("c", 5), n("d", 6)
	step("slice")
	// The slice is a call too.
	list("a")[n("b", 2)], list("c")[n("d", 3)] = n("e", 7), n("f", 8)
	step("slice call")
	// Map elements.
	m[key("k1")], m[key("k2")] = n("c", 1), n("d", 2)
	step("map")
	table("a")[key("k3")], table("c")[key("k1")] = n("e", 3), n("f", 4)
	step("map call")
	// Fields through a pointer, and pointers.
	item("a").Price, item("b").Price = n("c", 10), n("d", 20)
	step("field")
	*ptr("a"), *ptr("b") = n("c", 1), n("d", 2)
	step("pointer")
	// A name and an element; an element and a name.
	var x int
	x, xs[n("a", 0)] = n("b", 3), n("c", 4)
	xs[n("d", 1)], x = n("e", 5), n("f", 6)
	step("name" + strconv.Itoa(x))
	// One call with two values.
	two := func(name string) (int, int) {
		log = append(log, name)
		return 11, 12
	}
	xs[n("a", 2)], m[key("k2")] = two("c")
	step("two values")
	// An operation and an assignment.
	xs[n("a", 3)] += n("b", 100)
	m[key("k3")] += n("d", 100)
	list("e")[n("f", 0)] -= n("g", 1)
	item("h").Price *= n("i", 3)
	*ptr("j") += n("k", 5)
	table("l")[key("k1")] *= n("m", 2)
	step("operation")
	// The index of the left side is read before a store changes its name.
	i := 0
	i, xs[i] = 1, 9
	xs[i], i = 8, 2
	xs[0], xs[1] = xs[1], xs[0]
	step("index" + strconv.Itoa(i))
	// A new name and a call on each side.
	c, d := n("a", 1), n("b", 2)
	step("define" + strconv.Itoa(c+d))
	// A store that panics: Go reads the right side first, so its calls
	// run, and a store to the left of the bad one is done.
	var none *Item
	var nilMap map[string]int
	log = append(log, try(func() { xs[n("a", 9)] = n("b", 1) }))
	log = append(log, try(func() { xs[n("a", 0)], xs[n("b", 9)] = n("c", 41), n("d", 42) }))
	log = append(log, try(func() { none.Price = n("a", 1) }))
	log = append(log, try(func() { p, none.Price, q = n("a", 51), n("b", 52), n("c", 53) }))
	log = append(log, try(func() { *ptr("a"), nilMap[key("b")] = n("c", 61), n("d", 62) }))
	log = append(log, try(func() { xs[n("a", 9)] += n("b", 1) }))
	log = append(log, try(func() { list("a")[n("b", 1)], list("c")[n("d", -1)] = n("e", 71), n("f", 72) }))
	step("panic")
	return strings.Join(log, " ")
}

// Places gives the variable that each store of an assignment changes. Go
// reads the slice, the map and the pointer of the left side before a store
// of the same statement gives the name a new value.
func Places() string {
	var log []string
	n := func(name string, v int) int {
		log = append(log, name)
		return v
	}
	// The name of the slice, of the map and of the pointer changes in the
	// same statement: the store goes to the first value.
	xs, other := []int{0, 0}, []int{5, 5}
	xs, xs[0] = other, 1
	m, m2 := map[string]int{}, map[string]int{}
	firstMap := m
	m, m["k"] = m2, 2
	a, b := Item{Name: "a"}, Item{Name: "b"}
	ptr := &a
	ptr, ptr.Price = &b, 3
	p, q := 0, 0
	pp := &p
	pp, *pp = &q, 4
	log = append(log, fmt.Sprint(xs, other, len(m), firstMap["k"], a.Price, b.Price, ptr.Name, p, q, *pp))

	// An embedded pointer is an operand of the selector.
	w := Wrapped{Item: &a}
	w.Item, w.Price = &b, 30
	w.Name, w.Count = "w", 2
	log = append(log, fmt.Sprint(a.Price, b.Price, b.Name, w.Count))

	// Arrays, and elements that are structs or slices.
	var arr [3]int
	arr[n("a", 0)], arr[n("b", 2)] = n("c", 7), n("d", 8)
	parr := &arr
	parr[n("e", 1)] = n("f", 9)
	rows := []Item{{Name: "r0"}, {Name: "r1"}}
	rows[n("g", 1)].Price, rows[n("h", 0)].Price = n("i", 11), n("j", 12)
	grid := [][]int{{0, 0}, {0, 0}}
	grid[n("k", 1)][n("l", 0)], grid[n("m", 0)][n("o", 1)] = n("p", 13), n("q", 14)
	var cells [2][2]int
	cells[n("r", 1)][n("s", 1)] = n("t", 15)
	wrapped := []Wrapped{{Item: &a}}
	wrapped[n("u", 0)].Price = n("v", 16)
	log = append(log, fmt.Sprint(arr, rows, grid, cells, a.Price))

	// An operation and an assignment keeps the type of the left side.
	f, u, text := 1.5, uint8(3), "t"
	f += 1
	f *= float64(n("a", 2))
	u <<= 2
	u |= 1
	rows[n("b", 0)].Name += text + "!"
	rows[0].Tags = append(rows[0].Tags, "x")
	rows[n("c", 0)].Tags[n("d", 0)] += "y"
	arr[n("e", 1)]++
	grid[n("f", 0)][n("g", 0)]--
	cells[1][1] %= 4
	w.Price /= 3
	w.Count -= n("h", 5)
	*pp ^= 6
	log = append(log, fmt.Sprint(f, u, rows[0].Name, rows[0].Tags, arr, grid, cells, b.Price, w.Count, q))

	// A panic of the left side comes after the calls of the right side.
	var none *Wrapped
	var noArr *[3]int
	empty := Wrapped{}
	log = append(log, try(func() { arr[n("a", 3)] = n("b", 1) }))
	log = append(log, try(func() { noArr[n("a", 0)] = n("b", 1) }))
	log = append(log, try(func() { none.Count = n("a", 1) }))
	log = append(log, try(func() { empty.Price = n("a", 1) }))
	log = append(log, try(func() { rows[n("a", 5)].Price = n("b", 1) }))
	log = append(log, try(func() { rows[n("a", 0)].Price, none.Count = n("b", 21), n("c", 22) }))
	log = append(log, fmt.Sprint(arr, rows[0].Price))
	return strings.Join(log, " ")
}

// Makes gives the length and the capacity of each slice that make returns,
// and the sizes that are a panic in Go: a capacity below the length, and a
// negative size.
func Makes() string {
	var log []string
	size := func(v int) int { return v }
	a := make([]int, size(2))
	b := make([]string, size(2), size(5))
	c := make([]Item, size(3), size(3))
	d := make([]int, size(0), size(0))
	log = append(log, fmt.Sprint(len(a), cap(a), len(b), cap(b), len(c), cap(c), len(d), cap(d)))
	log = append(log, try(func() { _ = make([]int, size(3), size(2)) }))
	log = append(log, try(func() { _ = make([]int, size(1), size(0)) }))
	log = append(log, try(func() { _ = make([]int, size(-1)) }))
	log = append(log, try(func() { _ = make([]int, size(0), size(-1)) }))
	log = append(log, try(func() { _ = make([]int, size(2), size(2)) }))
	return strings.Join(log, " ")
}
