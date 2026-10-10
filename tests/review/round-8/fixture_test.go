package round8_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/propgen"
	cartroute "github.com/alternayte/gx/tests/review/round-8/fix/cart/route"
	"github.com/alternayte/gx/tests/review/round-8/fix/home"
	productsroute "github.com/alternayte/gx/tests/review/round-8/fix/products/route"
)

// propgen.Fixture writes the source of a fixture for the two callers of the
// release: the capture of the dev client (REQ-AI-12, props_gxdev.go) and
// the printed fixture of gx fuzz (REQ-AI-11, fuzz_gxdev.go). Both give the
// source to the user with no change.

// clockProps are the props of a component that shows a time of day:
//
//	<time>{p.At.Format("15:04")}</time>
type clockProps struct {
	At time.Time
}

// TestREQ_AI_12_FixtureKeepsTheTimeOfDay checks REQ-AI-12: "The dev client
// has an action that saves the props of a component of the current page as
// a named fixture", with the acceptance "capture on the product page of the
// shop adds a fixture; the gallery shows it with the same HTML as the
// page."
//
// The page shows a time prop in the zone of the value: 18:30 in a zone one
// hour east of UTC. The fixture holds the instant in UTC, so the gallery
// shows 17:30. The saved props are not the props of the page. A time of a
// loader has the zone of the server or of the database driver, and a dev
// machine is seldom in UTC.
func TestREQ_AI_12_FixtureKeepsTheTimeOfDay(t *testing.T) {
	props := clockProps{At: time.Date(2026, 3, 1, 18, 30, 0, 0, time.FixedZone("CET", 3600))}
	src, imports, err := propgen.Fixture(reflect.ValueOf(props), reflect.TypeOf(props).PkgPath(), propgen.Hooks{})
	if err != nil {
		// A save that stops with a message saves no wrong value.
		return
	}
	out, ok := runProgram(t, "package main\n\n"+importBlock(imports, "fmt")+
		"\ntype clockProps struct {\n\tAt time.Time\n}\n\nvar fixture = clockProps"+src+"\n\n"+
		"func main() { fmt.Print(fixture.At.Format(\"15:04\")) }\n")
	if !ok {
		t.Fatalf("the fixture %s does not compile:\n%s", src, out)
	}
	if want := props.At.Format("15:04"); out != want {
		t.Errorf("the page shows the time %s, and the saved fixture %s shows %s: the gallery does not have the HTML of the page", want, src, out)
	}
}

// panelProps are the props of a component that shows the error of a load:
//
//	if p.Err != nil { <p role="alert">{p.Err.Error()}</p> }
type panelProps struct {
	Err error
}

// TestREQ_AI_12_FixtureWithATypeThatIsNotExported checks REQ-AI-12: "A prop
// that Go source cannot hold (a function, a channel) stops the save with a
// message."
//
// The prop holds context.DeadlineExceeded: the load of the page timed out.
// Its type, context.deadlineExceededError, is not exported, so the source
// of a different package cannot name it. The save does not stop: the
// fixture is `{Err: context.deadlineExceededError{}}`. The capture writes it
// into the fixtures file, and the app does not build again.
func TestREQ_AI_12_FixtureWithATypeThatIsNotExported(t *testing.T) {
	props := panelProps{Err: context.DeadlineExceeded}
	src, imports, err := propgen.Fixture(reflect.ValueOf(props), reflect.TypeOf(props).PkgPath(), propgen.Hooks{})
	if err != nil {
		// The save stops with a message.
		return
	}
	out, ok := runProgram(t, "package main\n\n"+importBlock(imports)+
		"\ntype panelProps struct {\n\tErr error\n}\n\nvar fixture = panelProps"+src+"\n\n"+
		"func main() { _ = fixture }\n")
	if !ok {
		t.Errorf("the save does not stop, and the fixture %s is not Go source that compiles:\n%s", src, out)
	}
}

// TestREQ_AI_12_FixtureWithTwoPackagesOfOneName checks REQ-AI-12: "The dev
// client has an action that saves the props of a component of the current
// page as a named fixture in its `<Name>.fixtures.go` file."
//
// DR-01 gives each slice a package with the name route. The props of the
// component hold a type of the route package of the cart and a type of the
// route package of the products. The fixture names both types as `route.X`
// and gives the two import paths. No file can import the two packages under
// the one name, so the fixtures file with the entry does not compile. A
// second case of the same defect: the fixtures file imports the package
// under a different name, and addFixture (internal/devserver/capture.go)
// adds no import for a path that the file has.
func TestREQ_AI_12_FixtureWithTwoPackagesOfOneName(t *testing.T) {
	props := home.ShellProps{Add: cartroute.Add{SKU: "tee"}, Show: productsroute.Show{ID: "7"}}
	src, imports, err := propgen.Fixture(reflect.ValueOf(props), reflect.TypeOf(props).PkgPath(), propgen.Hooks{})
	if err != nil {
		// The save stops with a message.
		return
	}
	// The entry in a file of the package of the component, as the
	// capture writes it into Shell.fixtures.go.
	file := "package home\n\n" + importBlock(imports) + "\nvar shellFixtures = map[string]ShellProps{\n\t\"Captured\": " + src + ",\n}\n"
	pkgDir := filepath.Join(repoRoot(t), "tests", "review", "round-8", "fix", "home")
	if out, ok := buildWithFile(t, pkgDir, "shell_fixtures.go", file); !ok {
		t.Errorf("the fixtures file with the saved entry does not compile.\nentry: %s\nimports: %v\n%s", src, imports, out)
	}
}
