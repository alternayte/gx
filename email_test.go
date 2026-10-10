package gx

import (
	"strings"
	"testing"
)

func emailAttrs(kv ...string) Attrs {
	var out Attrs
	for i := 0; i+1 < len(kv); i += 2 {
		kind := AttrText
		if kv[i] == "href" || kv[i] == "src" {
			kind = AttrURL
		}
		out = append(out, Attr{Key: kv[i], Value: kv[i+1], Kind: kind})
	}
	return out
}

// TestREQ_REG_16_RenderEmail covers the document, the absolute URLs, the
// token values and the plain text of gx.RenderEmail on a tree that a person
// wrote. The golden email of the shop covers generated code.
func TestREQ_REG_16_RenderEmail(t *testing.T) {
	node := Frag(
		El("h1", emailAttrs("style", "color:var(--primary);font-size:var(--missing, 24px)"), Text("Order 1042")),
		El("p", nil, Text("Thank you, Ada & Co."), El("br", nil), Text("We have your order.")),
		El("table", emailAttrs("role", "presentation"),
			El("tr", nil, El("td", nil, Text("Tea")), El("td", nil, Text("2"))),
			El("tr", nil, El("td", nil, Text("Cup")), El("td", nil, Text("1"))),
		),
		El("hr", nil),
		El("p", nil,
			El("a", Attrs{{Key: "href", Value: "/orders/1042?tab=a&b=1", Kind: AttrURL, Active: "page"}}, Text("See the order")),
			Text(" or "),
			El("a", emailAttrs("href", "https://help.example/x"), Text("https://help.example/x")),
		),
		El("img", emailAttrs("src", "/logo.png", "alt", "Acme", "width", "40")),
		El("img", emailAttrs("src", "//cdn.example/x.png", "alt", "")),
	)
	msg, err := RenderEmail(node, EmailOptions{BaseURL: "https://shop.example/", Tokens: map[string]string{"--primary": "#171717"}, Title: "Your order <1042>"})
	if err != nil {
		t.Fatal(err)
	}
	wantHTML := `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>Your order &lt;1042&gt;</title></head><body style="margin:0;padding:0">` +
		`<h1 style="color:#171717;font-size:24px">Order 1042</h1>` +
		`<p>Thank you, Ada &amp; Co.<br>We have your order.</p>` +
		`<table role="presentation"><tr><td>Tea</td><td>2</td></tr><tr><td>Cup</td><td>1</td></tr></table><hr>` +
		`<p><a href="https://shop.example/orders/1042?tab=a&amp;b=1">See the order</a> or <a href="https://help.example/x">https://help.example/x</a></p>` +
		`<img src="https://shop.example/logo.png" alt="Acme" width="40"><img src="//cdn.example/x.png" alt="">` +
		`</body></html>`
	if msg.HTML != wantHTML {
		t.Errorf("HTML:\n got %s\nwant %s", msg.HTML, wantHTML)
	}
	wantText := "Order 1042\n\nThank you, Ada & Co.\nWe have your order.\n\nTea 2\nCup 1\n\n----------\n\n" +
		"See the order (https://shop.example/orders/1042?tab=a&b=1) or https://help.example/x\n\nAcme\n"
	if msg.Text != wantText {
		t.Errorf("text:\n got %q\nwant %q", msg.Text, wantText)
	}

	// A node with its own html element stays the document.
	own, err := RenderEmail(Frag(Raw("<!DOCTYPE html>"), El("html", emailAttrs("lang", "de"),
		El("head", nil, El("title", nil, Text("Titel")), El("style", nil, Raw("a > b { color: red }"))),
		El("body", nil, El("p", nil, Text("Hallo"))))), EmailOptions{BaseURL: "https://shop.example"})
	if err != nil {
		t.Fatal(err)
	}
	if own.HTML != `<!DOCTYPE html><html lang="de"><head><title>Titel</title><style>a > b { color: red }</style></head><body><p>Hallo</p></body></html>` || own.Text != "Hallo\n" {
		t.Errorf("own document:\n%s\n%q", own.HTML, own.Text)
	}

	for _, base := range []string{"", "shop.example", "/shop", "ftp://x"} {
		if _, err := RenderEmail(Text("x"), EmailOptions{BaseURL: base}); err == nil || !strings.Contains(err.Error(), "BaseURL") {
			t.Errorf("BaseURL %q: err = %v", base, err)
		}
	}
	if _, err := RenderEmail(El("p", emailAttrs("style", "color:var(--primary)"), Text("x")), EmailOptions{BaseURL: "https://shop.example"}); err == nil || !strings.Contains(err.Error(), "--primary") {
		t.Errorf("a token with no value: err = %v", err)
	}
}

// TestREQ_REG_17_RenderEmailErrors covers the error result of
// gx.RenderEmail for each construct that an email cannot hold.
func TestREQ_REG_17_RenderEmailErrors(t *testing.T) {
	opt := EmailOptions{BaseURL: "https://shop.example"}
	for want, node := range map[string]Node{
		"a class attribute on <p>": El("p", emailAttrs("class", "text-sm"), Text("x")),
		"a script element":         El("div", nil, El("script", nil, Raw("alert(1)"))),
		"a signal":                 El("div", emailAttrs("data-signals", `{"a":1}`), Text("x")),
		"a client expression or an action invocation": El("button", Attrs{On("click", "POST", "/cart/add", "")}, Text("Add")),
		"an island": El(islandElement, nil),
	} {
		_, err := RenderEmail(node, opt)
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "GX6010") {
			t.Errorf("%s: err = %v", want, err)
		}
	}
	// A class in trusted raw HTML is an error too: the check reads what
	// the email holds.
	if _, err := RenderEmail(Raw(`<p class="a">x</p>`), opt); err == nil || !strings.Contains(err.Error(), "a class attribute") {
		t.Errorf("a class in raw HTML: err = %v", err)
	}
	if _, err := RenderEmail(El("p", emailAttrs("style", "color:#111"), Text("x")), opt); err != nil {
		t.Errorf("a node with a style only: %v", err)
	}
}
