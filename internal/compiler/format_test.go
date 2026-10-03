package compiler_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const fmtCard = `package card

props {
  Title    string
  Variant  Variant = Default
  Header   gx.Node  = nil
  Children gx.Node
  Attrs    gx.Attrs = nil
}

<article class="rounded-xl border p-4"
         class:shadow-lg={p.Variant == Raised}
         {...p.Attrs}>
  if p.Header != nil {
    <header>{p.Header}</header>
  } else if p.Title != "" {
    <h3 class="font-semibold">{p.Title}</h3>
  } else {
    <em>none</em>
  }
  {p.Children}
  <script>if (a < b) { go() }
const x = "</div>";</script>
  <input disabled>
  <img src="/x.png" alt="">
  <!-- keep -->
  {/* drop */}
  switch p.Variant {
    case Raised:
      <b>raised</b>
    default:
      <b>flat</b>
  }
</article>
`

const fmtCart = `package cart

import "app/ui/card"

props {
  Items   []Item
  History []Point
  Row     gx.Slot[Item] = DefaultRow
}
signals { Qty int = 1 }

<card.Card title="Cart" variant={card.Raised}>
  <:header>Your cart ({len(p.Items)})</:header>

  total := sum(p.Items)
  for _, it := range p.Items {
    <div #row(it Item)>{p.Row(it)}</div>
  }
  <span #total(total money.Amount)>{total}</span>

  <input type="number" bind:value={$Qty} />
  <p show={$Qty > len(p.Items)}>Over stock</p>
  <button on:click={Checkout{}}>Pay</button>
  <RevenueChart data={p.History} load="visible" />
</card.Card>
`

func canon(s string) string { return strings.Join(strings.Fields(s), " ") }

func dumpAttrs(as []compiler.Attr) string {
	parts := make([]string, 0, len(as))
	for _, a := range as {
		switch a.Kind {
		case compiler.AttrString:
			parts = append(parts, a.Name+"="+canon(a.Value))
		case compiler.AttrExpr:
			parts = append(parts, a.Name+"={"+canon(a.Value)+"}")
		case compiler.AttrSpread:
			parts = append(parts, "...{"+canon(a.Value)+"}")
		case compiler.AttrFragment:
			parts = append(parts, "#"+a.Name+"("+canon(a.Value)+")")
		default:
			parts = append(parts, a.Name)
		}
	}
	return strings.Join(parts, " ")
}

func dumpNodes(ns []compiler.Node) []string {
	var out []string
	for _, n := range ns {
		switch t := n.(type) {
		case *compiler.Text:
			if strings.TrimSpace(t.Data) == "" {
				continue
			}
			out = append(out, "text:"+t.Data)
		case *compiler.Expr:
			out = append(out, "expr:"+canon(t.Data))
		case *compiler.Comment:
			out = append(out, "comment:"+t.Data)
		case *compiler.HTMLComment:
			out = append(out, "html:"+t.Data)
		case *compiler.Let:
			out = append(out, "let:"+t.Name+":="+canon(t.Expr))
		case *compiler.Element:
			out = append(out, "el:"+t.Name+"("+dumpAttrs(t.Attrs)+")")
			if t.HasRaw {
				out = append(out, "raw:"+t.RawText)
			}
			out = append(out, dumpNodes(t.Children)...)
			out = append(out, "/el:"+t.Name)
		case *compiler.Control:
			out = append(out, "ctl:"+t.Kind+":"+canon(t.Header))
			out = append(out, dumpNodes(t.Body)...)
			if len(t.Else) > 0 {
				out = append(out, "else")
				out = append(out, dumpNodes(t.Else)...)
			}
			for _, c := range t.Cases {
				out = append(out, "case:"+canon(c.Header))
				out = append(out, dumpNodes(c.Body)...)
			}
			out = append(out, "/ctl:"+t.Kind)
		}
	}
	return out
}

func dumpFields(fs []compiler.Field) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Name+" "+canon(f.Type)+" = "+canon(f.Default))
	}
	return out
}

func TestREQ_AUT_17_FmtCanonicalAndIdempotent(t *testing.T) {
	for i, src := range []string{fmtCard, fmtCart} {
		out, diags := compiler.FormatSource("Card.gx", []byte(src))
		if len(diags) > 0 {
			t.Fatalf("case %d: diagnostics = %v", i, diags)
		}
		out2, diags := compiler.FormatSource("Card.gx", out)
		if len(diags) > 0 {
			t.Fatalf("case %d second pass: diagnostics = %v", i, diags)
		}
		if string(out) != string(out2) {
			t.Fatalf("case %d is not idempotent:\nfirst:\n%s\nsecond:\n%s", i, out, out2)
		}
	}
}

func TestREQ_AUT_17_FmtPreservesStructure(t *testing.T) {
	for i, src := range []string{fmtCard, fmtCart} {
		f1, diags := compiler.ParseFile("Card.gx", []byte(src))
		if len(diags) > 0 || f1 == nil {
			t.Fatalf("case %d: diagnostics = %v", i, diags)
		}
		out := compiler.Format(f1)
		f2, diags := compiler.ParseFile("Card.gx", out)
		if len(diags) > 0 || f2 == nil {
			t.Fatalf("case %d reparse: diagnostics = %v", i, diags)
		}
		if got, want := dumpNodes(f2.Body), dumpNodes(f1.Body); !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d body changed:\nwant %v\ngot  %v", i, want, got)
		}
		if got, want := dumpFields(f2.Props), dumpFields(f1.Props); !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d props changed: want %v got %v", i, want, got)
		}
		if got, want := dumpFields(f2.Signals), dumpFields(f1.Signals); !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d signals changed: want %v got %v", i, want, got)
		}
	}
}
