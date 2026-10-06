package gx

import (
	"encoding/json"
	"math"
	"math/rand"
	"testing"
	"time"
	"unicode/utf8"
)

// The props of an island have the JSON form of encoding/json, written with
// no reflection.
func TestREQ_ISL_02_AppendJSONMatchesEncodingJSON(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	strs := []string{"", "plain", `"quoted" \ back`, "<script>&amp;</script>", "\x00\x1f\n\r\t", "é 日本 🙂", "  ", "bad \xff byte \xc3", "'single'"}
	for i := 0; i < 2000; i++ {
		b := make([]byte, rng.Intn(12))
		for j := range b {
			b[j] = byte(rng.Intn(256))
		}
		strs = append(strs, string(b))
		r := make([]rune, rng.Intn(6))
		for j := range r {
			r[j] = rune(rng.Intn(utf8.MaxRune))
		}
		strs = append(strs, string(r))
	}
	for _, s := range strs {
		want, _ := json.Marshal(s)
		if got := AppendJSONString(nil, s); string(got) != string(want) {
			t.Fatalf("AppendJSONString(%q) = %s, want %s", s, got, want)
		}
	}
	floats := []float64{0, math.Copysign(0, -1), 1, -1.5, 1e-6, 9.99e-7, 1e-7, 1e20, 1e21, 1.5e300, 5e-324, math.MaxFloat64, 0.1, 100, 123456789.125}
	for i := 0; i < 2000; i++ {
		floats = append(floats, math.Float64frombits(rng.Uint64()))
	}
	for _, f := range floats {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		want, _ := json.Marshal(f)
		if got := AppendJSONFloat(nil, f); string(got) != string(want) {
			t.Fatalf("AppendJSONFloat(%v) = %s, want %s", f, got, want)
		}
	}
	for _, tm := range []time.Time{{}, time.Unix(0, 0).UTC(), time.Date(2026, 10, 6, 1, 2, 3, 456, time.FixedZone("x", -5*3600))} {
		want, _ := json.Marshal(tm)
		if got := AppendJSONTime(nil, tm); string(got) != string(want) {
			t.Fatalf("AppendJSONTime(%v) = %s, want %s", tm, got, want)
		}
	}
	if got := string(AppendJSONInt(AppendJSONUint(AppendJSONBool(nil, true), 18446744073709551615), -9223372036854775808)); got != "true18446744073709551615-9223372036854775808" {
		t.Fatalf("numbers = %s", got)
	}
}

// JSON has no NaN and no infinity, and the browser rounds an integer past
// 53 bits. Dev stops on both; production writes null and the integer.
func TestREQ_ISL_02_AppendJSONDevChecks(t *testing.T) {
	if got := string(AppendJSONFloat(nil, math.NaN())); got != "null" {
		t.Fatalf("NaN = %s, want null", got)
	}
	SetDev(true)
	defer SetDev(false)
	for name, fn := range map[string]func(){
		"NaN":  func() { AppendJSONFloat(nil, math.NaN()) },
		"Inf":  func() { AppendJSONFloat(nil, math.Inf(1)) },
		"int":  func() { AppendJSONInt(nil, 1<<53) },
		"uint": func() { AppendJSONUint(nil, 1<<53) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic in dev", name)
				}
			}()
			fn()
		}()
	}
	AppendJSONInt(nil, 1<<53-1)
}

func TestREQ_ISL_01_IslandElement(t *testing.T) {
	got := String(Island("app/dash/Chart", `{"a":"<b>"}`))
	want := `<gx-island name="app/dash/Chart" props="{&#34;a&#34;:&#34;&lt;b&gt;&#34;}"><div data-gx-island-root data-ignore-morph></div></gx-island>`
	if got != want {
		t.Fatalf("island = %s, want %s", got, want)
	}
}
