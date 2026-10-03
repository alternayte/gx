package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func collectControls(ns []compiler.Node) []*compiler.Control {
	var out []*compiler.Control
	for _, n := range ns {
		switch t := n.(type) {
		case *compiler.Element:
			out = append(out, collectControls(t.Children)...)
		case *compiler.Control:
			out = append(out, t)
			out = append(out, collectControls(t.Body)...)
			out = append(out, collectControls(t.Else)...)
			for _, c := range t.Cases {
				out = append(out, collectControls(c.Body)...)
			}
		}
	}
	return out
}

func collectLets(ns []compiler.Node) []*compiler.Let {
	var out []*compiler.Let
	for _, n := range ns {
		switch t := n.(type) {
		case *compiler.Element:
			out = append(out, collectLets(t.Children)...)
		case *compiler.Let:
			out = append(out, t)
		case *compiler.Control:
			out = append(out, collectLets(t.Body)...)
			out = append(out, collectLets(t.Else)...)
			for _, c := range t.Cases {
				out = append(out, collectLets(c.Body)...)
			}
		}
	}
	return out
}

const controlFlowSrc = `package loop

<div>
  if a {
    <b>1</b>
  } else if b {
    <b>2</b>
  } else {
    <b>3</b>
  }
  for i := 0; i < 3; i++ {
    <i>{i}</i>
  }
  for _, it := range items {
    <li>{it}</li>
  }
  for k, v := range m {
    <li>{k}{v}</li>
  }
  for i := range 3 {
    <i>{i}</i>
  }
  for {
    <i>loop</i>
  }
  switch kind {
  case "a":
    <p>A</p>
  case "b", "c":
    <p>B</p>
  default:
    <p>D</p>
  }
  switch {
  case x:
    <i>1</i>
  }
  total := sum(items)
  a, b := f()
</div>
`

func TestREQ_AUT_05_ControlFlow(t *testing.T) {
	f, diags := compiler.ParseFile("loop/Loop.gx", []byte(controlFlowSrc))
	if len(diags) > 0 || f == nil {
		t.Fatalf("diagnostics = %v", diags)
	}
	controls := collectControls(f.Body)
	var ifs, fors, switches int
	for _, c := range controls {
		switch c.Kind {
		case "if":
			ifs++
		case "for":
			fors++
		case "switch":
			switches++
		}
	}
	if ifs != 2 || fors != 5 || switches != 2 {
		// One if plus one else-if; five for forms; two switches.
		t.Fatalf("control counts: if=%d for=%d switch=%d, want 2/5/2", ifs, fors, switches)
	}
	for _, c := range controls {
		if c.Kind == "if" && c.Header == "a" {
			if len(c.Else) == 0 {
				t.Fatal("if has no else branch")
			}
			if _, ok := c.Else[0].(*compiler.Control); !ok {
				t.Fatal("else if is not parsed as a nested if")
			}
		}
		if c.Kind == "switch" && c.Header == "kind" {
			if len(c.Cases) != 3 {
				t.Fatalf("switch cases = %d, want 3", len(c.Cases))
			}
			if !c.Cases[2].IsDefault {
				t.Fatal("last case is not default")
			}
		}
	}
	lets := collectLets(f.Body)
	if len(lets) != 2 {
		t.Fatalf("lets = %d, want 2", len(lets))
	}
	if lets[0].Name != "total" || lets[1].Name != "a, b" {
		t.Fatalf("let names = %q, %q", lets[0].Name, lets[1].Name)
	}

	out := compiler.Format(f)
	if _, diags := compiler.FormatSource("loop/Loop.gx", out); len(diags) > 0 {
		t.Fatalf("formatted control flow does not parse: %v\n%s", diags, out)
	}
}

func TestREQ_AUT_05_KeywordTextStaysText(t *testing.T) {
	src := "<p>if this is text it stays text</p>\n<p>for sale: 3</p>\n<p>switch to the list</p>\n"
	f, diags := compiler.ParseFragment([]byte(src))
	if len(diags) > 0 || f == nil {
		t.Fatalf("diagnostics = %v", diags)
	}
	var ps []*compiler.Element
	for _, n := range f.Body {
		if el, ok := n.(*compiler.Element); ok {
			ps = append(ps, el)
		}
	}
	if len(ps) != 3 {
		t.Fatalf("paragraphs = %d, want 3", len(ps))
	}
	for i, p := range ps {
		if len(p.Children) != 1 {
			t.Fatalf("paragraph %d has %d children, want 1 text node", i, len(p.Children))
		}
		txt, ok := p.Children[0].(*compiler.Text)
		if !ok {
			t.Fatalf("paragraph %d child is %T, want text", i, p.Children[0])
		}
		if !strings.HasPrefix(strings.TrimSpace(txt.Data), []string{"if", "for", "switch"}[i]) {
			t.Fatalf("paragraph %d text = %q", i, txt.Data)
		}
	}
}
