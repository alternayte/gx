package gx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/alternayte/gx"
)

// signalAdapter decodes a JSON body as signals, like the Datastar adapter.
type signalAdapter struct {
	body map[string]any
}

func (a *signalAdapter) Name() string             { return "signal-test" }
func (a *signalAdapter) Signals() bool            { return true }
func (a *signalAdapter) Runtime() gx.Node         { return nil }
func (a *signalAdapter) Assets() map[string][]byte { return nil }
func (a *signalAdapter) Respond(http.ResponseWriter, *http.Request, *gx.Response) error {
	return nil
}

func (a *signalAdapter) ReadSignals(r *http.Request, dst any) error {
	if a.body == nil {
		return nil
	}
	data, err := json.Marshal(a.body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func TestREQ_ACT_03_BindSignal(t *testing.T) {
	a := &signalAdapter{body: map[string]any{
		"cart": map[string]any{
			"Cart": map[string]any{"qty": float64(2)},
		},
	}}
	gx.SetAdapter(a)
	defer gx.SetAdapter(nil)

	req, _ := http.NewRequest("POST", "/act", nil)
	req.Header.Set("Gx-Scope", "cart.Cart")
	signals, err := gx.Signals(req)
	if err != nil {
		t.Fatal(err)
	}
	var qty int64
	if err := gx.BindSignal(signals, gx.Scope(req), "qty", &qty); err != nil {
		t.Fatal(err)
	}
	if qty != 2 {
		t.Fatalf("qty = %d, want 2", qty)
	}
}

func TestREQ_ACT_03_BindSignalMissing(t *testing.T) {
	a := &signalAdapter{body: map[string]any{"cart": map[string]any{"Cart": map[string]any{"open": true}}}}
	gx.SetAdapter(a)
	defer gx.SetAdapter(nil)

	req, _ := http.NewRequest("POST", "/act", nil)
	req.Header.Set("Gx-Scope", "cart.Cart")
	signals, err := gx.Signals(req)
	if err != nil {
		t.Fatal(err)
	}
	qty := int64(7)
	if err := gx.BindSignal(signals, gx.Scope(req), "qty", &qty); err != nil {
		t.Fatal(err)
	}
	if qty != 7 {
		t.Fatalf("missing signal changed the value to %d", qty)
	}
}

func TestREQ_ACT_03_BindSignalWrongScope(t *testing.T) {
	a := &signalAdapter{body: map[string]any{"cart": map[string]any{"Other": map[string]any{"qty": float64(9)}}}}
	gx.SetAdapter(a)
	defer gx.SetAdapter(nil)

	req, _ := http.NewRequest("POST", "/act", nil)
	req.Header.Set("Gx-Scope", "cart.Cart")
	signals, err := gx.Signals(req)
	if err != nil {
		t.Fatal(err)
	}
	qty := int64(0)
	if err := gx.BindSignal(signals, gx.Scope(req), "qty", &qty); err != nil {
		t.Fatal(err)
	}
	if qty != 0 {
		t.Fatalf("another instance's signal leaked: qty = %d", qty)
	}
}
