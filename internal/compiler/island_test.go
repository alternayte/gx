package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const chartIsland = `import type { Props } from "./RevenueChart.props";

export default (el: HTMLElement, { data }: Props) => {
  el.textContent = String(data.length);
};
`

const chartProps = `package dash

type Point struct {
	X int ` + "`json:\"x\"`" + `
	Y int ` + "`json:\"y\"`" + `
}

type RevenueChartProps struct {
	Data  []Point ` + "`json:\"data\"`" + `
	Title string  ` + "`json:\"title\"`" + `
}
`

const chartPage = `package dash

props {
  History []Point
}

<section>
  <RevenueChart data={p.History} title="Revenue" />
</section>
`

func TestREQ_ISL_01_MissingPropsStruct(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"dash/types.go":        "package dash\n\ntype Point struct{ X, Y int }\n",
		"dash/RevenueChart.ts": chartIsland,
		"dash/Page.gx":         "package dash\n\n<section><RevenueChart /></section>\n",
	})
	diags := compiler.Check(dir)
	d := diagWith(t, diags, compiler.CodeIslandProps)
	if d.Code != "GX6001" {
		t.Fatalf("code = %s, want GX6001", d.Code)
	}
	if !strings.Contains(d.Msg, `"RevenueChartProps"`) || !strings.Contains(d.Msg, `"RevenueChart"`) {
		t.Fatalf("GX6001 message = %q", d.Msg)
	}
	if !strings.HasSuffix(d.File, "RevenueChart.ts") || d.Line != 1 || d.Col != 1 {
		t.Fatalf("GX6001 position = %s:%d:%d, want RevenueChart.ts:1:1", d.File, d.Line, d.Col)
	}
	if d.Fix == "" {
		t.Fatal("GX6001 has no fix")
	}
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want only GX6001", diags)
	}
}

func TestREQ_ISL_01_MissingPropsStructWithoutGxFile(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"dash/types.go":        "package dash\n\ntype Point struct{ X, Y int }\n",
		"dash/RevenueChart.ts": chartIsland,
	})
	diags := compiler.Check(dir)
	diagWith(t, diags, compiler.CodeIslandProps)
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want only GX6001", diags)
	}
}

func TestREQ_ISL_01_IslandIsAComponent(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"dash/charts.go":       chartProps,
		"dash/RevenueChart.ts": chartIsland,
		"dash/Page.gx":         chartPage,
		"home/Home.gx":         "package home\n\nimport \"app/dash\"\n\n<main><dash.RevenueChart title=\"All\" /></main>\n",
	})
	if diags := compiler.Check(dir); len(diags) != 0 {
		t.Fatalf("island tag: unexpected diagnostics %v", diags)
	}
}

func TestREQ_ISL_01_IslandPropsAreChecked(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"dash/charts.go":       chartProps,
		"dash/RevenueChart.ts": chartIsland,
		"dash/Page.gx":         "package dash\n\n<section>\n  <RevenueChart colour=\"red\" />\n  <RevenueChart data=\"x\" />\n  <RevenueChart>text</RevenueChart>\n</section>\n",
	})
	diags := compiler.Check(dir)
	if len(diags) != 3 {
		t.Fatalf("diagnostics = %v, want three", diags)
	}
	if d := diags[0]; d.Code != compiler.CodeUnknownAttr || d.Line != 4 || !strings.Contains(d.Msg, `"colour"`) {
		t.Fatalf("first diagnostic = %v, want GX2003 for colour on line 4", d)
	}
	if d := diags[1]; d.Code != compiler.CodeStaticStringProp || d.Line != 5 {
		t.Fatalf("second diagnostic = %v, want GX2004 on line 5", d)
	}
	if d := diags[2]; d.Code != compiler.CodeUnknownAttr || d.Line != 6 || !strings.Contains(d.Msg, "Children") {
		t.Fatalf("third diagnostic = %v, want GX2003 for children on line 6", d)
	}
}

// A .ts file is an island only when its name is an exported identifier and
// it has a default export. Other TypeScript files in a Go package are plain
// modules that an island imports.
func TestREQ_ISL_01_OnlyDefaultExportIsAComponent(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":                     moduleWithGx(t),
		"dash/charts.go":             chartProps,
		"dash/Scale.ts":              "// export default is not here\nexport const note = `export default`;\nexport function scale(n: number) { return n * 2; }\n",
		"dash/util.ts":               "export default function util() {}\n",
		"dash/RevenueChart.props.ts": "export default {};\n",
		"dash/RevenueChart.ts":       chartIsland,
		"dash/Page.gx":               "package dash\n\n<section><Scale /></section>\n",
	})
	diags := compiler.Check(dir)
	d := diagWith(t, diags, compiler.CodeUnknownComponent)
	if !strings.Contains(d.Msg, `"Scale"`) {
		t.Fatalf("GX2002 message = %q", d.Msg)
	}
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want only GX2002", diags)
	}
}

func TestREQ_ISL_01_DefaultExportForms(t *testing.T) {
	for name, src := range map[string]string{
		"function":  "export default function mount(el: HTMLElement) {}\n",
		"arrow":     "const a = 1;\nexport default (el: HTMLElement) => {};\n",
		"named":     "function mount(el: HTMLElement) {}\nexport { mount as default };\n",
		"multiline": "function mount(el: HTMLElement) {}\nexport {\n  mount as default,\n};\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := writeTree(t, map[string]string{
				"go.mod":         moduleWithGx(t),
				"dash/types.go":  "package dash\n",
				"dash/Widget.ts": src,
			})
			diags := compiler.Check(dir)
			diagWith(t, diags, compiler.CodeIslandProps)
		})
	}
}

func TestREQ_ISL_01_DescribeListsIslands(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"dash/charts.go":       chartProps,
		"dash/RevenueChart.ts": chartIsland,
		"dash/Page.gx":         chartPage,
	})
	model, diags := compiler.Describe(dir)
	if len(diags) != 0 {
		t.Fatalf("describe: %v", diags)
	}
	if len(model.Islands) != 1 {
		t.Fatalf("islands = %+v, want one", model.Islands)
	}
	got := model.Islands[0]
	if got.Name != "RevenueChart" || got.Package != "app/dash" || got.File != "dash/RevenueChart.ts" {
		t.Fatalf("island = %+v", got)
	}
	if len(got.Props) != 2 || got.Props[0].Name != "Data" || got.Props[0].Type != "[]Point" || got.Props[1].Name != "Title" {
		t.Fatalf("island props = %+v", got.Props)
	}
}

// A new island file changes the file set, so the session runs a new
// analysis. An edit of the mount function changes nothing the analysis
// reads, so the session keeps its result.
func TestREQ_ISL_01_SessionSeesIslandFiles(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"dash/charts.go": chartProps,
		"dash/Page.gx":   chartPage,
	})
	s := compiler.NewSession()
	_, diags := s.Model(dir)
	diagWith(t, diags, compiler.CodeUnknownComponent)

	island := filepath.Join(dir, "dash", "RevenueChart.ts")
	if err := os.WriteFile(island, []byte(chartIsland), 0o644); err != nil {
		t.Fatal(err)
	}
	m, diags := s.Model(dir)
	if len(diags) != 0 {
		t.Fatalf("after the island file: %v", diags)
	}
	found := false
	for _, c := range m.Components {
		if c.Name == "RevenueChart" && c.PkgPath == "app/dash" && c.File.File == island {
			found = len(c.Props) == 2
		}
	}
	if !found {
		t.Fatalf("the model has no island component: %+v", m.Components)
	}

	if err := os.WriteFile(island, []byte(chartIsland+"\n// edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, diags := s.Model(dir); len(diags) != 0 {
		t.Fatalf("after an edit of the mount function: %v", diags)
	}
	if err := os.WriteFile(island, []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diags = s.Model(dir)
	diagWith(t, diags, compiler.CodeUnknownComponent)
}
