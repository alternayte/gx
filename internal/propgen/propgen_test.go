package propgen

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"
)

type line struct {
	SKU   string
	Qty   int
	notes string
}

type props struct {
	Title string
	Lines []line
	Tags  map[string]int
	Best  *line
	Count *int
	When  time.Time
	Any   any
	Do    func()
	C     chan int
}

func set(seed uint64, index int) reflect.Value {
	a, b := Seed(seed, "example.com/app/ui", "Card", index)
	return Set(reflect.TypeFor[props](), rand.New(rand.NewPCG(a, b)), ModeOf(index), Hooks{})
}

// The first three sets hold the empty value, a long string and markup
// characters in each string, and one seed gives the same sets.
func TestREQ_AI_11_PropSets(t *testing.T) {
	if got := set(7, 0); !got.IsZero() {
		t.Fatalf("set 0 is not the zero value: %#v", got.Interface())
	}
	for index, want := range map[int]string{1: LongString, 2: MarkupString} {
		p := set(7, index).Interface().(props)
		if p.Title != want {
			t.Errorf("set %d: Title = %q", index, p.Title)
		}
		for _, l := range p.Lines {
			if l.SKU != want {
				t.Errorf("set %d: a string of a slice is %q", index, l.SKU)
			}
			if l.notes != "" {
				t.Errorf("set %d: a field that is not exported has a value", index)
			}
		}
	}
	differ := false
	for index := 0; index < 20; index++ {
		a, b := set(7, index), set(7, index)
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			t.Fatalf("set %d: two runs with one seed differ", index)
		}
		if !reflect.DeepEqual(a.Interface(), set(8, index).Interface()) {
			differ = true
		}
	}
	if !differ {
		t.Fatal("a different seed gives the same sets")
	}
}

// A prop value becomes Go source, and a value with no Go source is an
// error that names the prop.
func TestREQ_AI_12_FixtureSource(t *testing.T) {
	n := 3
	v := props{
		Title: `a "b"`,
		Lines: []line{{SKU: "x", Qty: 2}, {}},
		Tags:  map[string]int{"b": 2, "a": 1},
		Best:  &line{SKU: "y", notes: "n"},
		Count: &n,
		When:  time.Date(2026, 10, 10, 12, 0, 0, 0, time.FixedZone("x", 3600)),
		Any:   int64(4),
	}
	const pkg = "github.com/alternayte/gx/internal/propgen"
	src, imports, err := Fixture(reflect.ValueOf(v), pkg, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	want := `{Title: "a \"b\"", Lines: []line{line{SKU: "x", Qty: 2}, line{}}, Tags: map[string]int{"a": 1, "b": 2}, ` +
		`Best: &line{SKU: "y", notes: "n"}, Count: func() *int { v := int(3); return &v }(), ` +
		`When: time.Date(2026, 10, 10, 11, 0, 0, 0, time.UTC), Any: int64(4)}`
	if src != want {
		t.Errorf("source:\n got %s\nwant %s", src, want)
	}
	if strings.Join(imports, ",") != "time" {
		t.Errorf("imports = %v", imports)
	}
	// From a different package, the type has its package name and the
	// field that is not exported has no source.
	if _, _, err := Fixture(reflect.ValueOf(v), "example.com/other", Hooks{}); err == nil || !strings.Contains(err.Error(), "Best.notes") {
		t.Errorf("a field of a different package that is not exported: err = %v", err)
	}
	src, imports, err = Fixture(reflect.ValueOf(props{Lines: []line{{SKU: "x"}}}), "example.com/other", Hooks{})
	if err != nil || src != `{Lines: []propgen.line{propgen.line{SKU: "x"}}}` || strings.Join(imports, ",") != pkg {
		t.Errorf("from a different package: %s, %v, %v", src, imports, err)
	}
	for name, bad := range map[string]props{
		"a function": {Do: func() {}},
		"a channel":  {C: make(chan int)},
	} {
		_, _, err := Fixture(reflect.ValueOf(bad), pkg, Hooks{})
		if err == nil || !strings.Contains(err.Error(), "Go source cannot hold "+name) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
