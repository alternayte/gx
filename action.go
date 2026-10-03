package gx

import (
	"net/http"
)

// action is the typed action handler built by Action (REQ-ACT-01).
type action[In any] struct {
	pattern string
	bind    func(*http.Request) (In, error)
	fn      func(*Ctx, In) error
}

// Action registers a handler for a route type of any method. The handler
// answers with Patch, SetSignals, Redirect, Toast or nothing.
func Action[In any](fn func(*Ctx, In) error) *action[In] {
	var zero In
	b, ok := any(&zero).(Binder)
	if !ok {
		panic("gx: Action input type needs generated Pattern and Bind methods")
	}
	pattern := b.Pattern()
	bind := func(r *http.Request) (In, error) {
		var in In
		err := any(&in).(Binder).Bind(r)
		return in, err
	}
	return &action[In]{pattern: pattern, bind: bind, fn: fn}
}

// Pattern implements Handler.
func (a *action[In]) Pattern() string { return a.pattern }

// ServeHTTP binds the input, runs the handler and sends the answer
// (REQ-ACT-10).
func (a *action[In]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	in, err := a.bind(r)
	if err != nil {
		renderError(w, r, &BindError{Err: err})
		return
	}
	ctx := &Ctx{W: w, R: r, res: &Response{}}
	handlerErr := a.fn(ctx, in)
	if handlerErr != nil {
		// A handler error shows a toast, and the dev overlay replaces
		// the page (REQ-ACT-10, REQ-DEV-06).
		ctx.res.Patches = append(ctx.res.Patches, ToastPatch{Text: handlerErr.Error()})
		ctx.res.Status = http.StatusInternalServerError
		ctx.res.Err = handlerErr
	}
	if len(ctx.res.Patches) == 0 {
		// An action with no answer answers 204 (REQ-ACT-10).
		w.WriteHeader(http.StatusNoContent)
		return
	}
	adapter := AdapterOf(r)
	if adapter == nil {
		http.Error(w, "gx: no adapter is set", http.StatusInternalServerError)
		return
	}
	if err := adapter.Respond(w, r, ctx.res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
