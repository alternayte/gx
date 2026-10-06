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
