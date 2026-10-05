package gx

import (
	"net/http"
)

// action is the typed action handler built by Action (REQ-ACT-01).
type action[In any] struct {
	pattern string
	bind    func(*http.Request) (In, error)
	fn      func(*Ctx, In) error
	// external is the base URL of the server that answers the action,
	// or "" when this app answers it (REQ-EXP-02).
	external string
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

// External marks the action as answered by another server at the base URL
// (REQ-EXP-02). Every invocation of the action then calls that server, so
// a static export can hold the page. The other server mounts the same
// action.
func (a *action[In]) External(base string) *action[In] {
	a.external = base
	registerExternal(a.pattern, base)
	return a
}

// exportFeature names the action for the export check (REQ-EXP-02).
func (a *action[In]) exportFeature() (kind string, serverOnly bool) {
	return "action", a.external == ""
}

// ServeHTTP binds the input, runs the handler and sends the answer
// (REQ-ACT-10).
func (a *action[In]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	in, err := a.bind(r)
	if err != nil {
		renderError(w, r, &BindError{Err: err})
		return
	}
	if violation := RunRules(&in); violation != nil {
		// A tampered signal value is a field error, not a handler
		// error (SI-13). M5 turns this into a form re-render.
		http.Error(w, violation.Error(), http.StatusUnprocessableEntity)
		return
	}
	ctx := &Ctx{W: w, R: r, res: &Response{}}
	handlerErr := a.fn(ctx, in)
	if handlerErr != nil {
		// A handler error shows a toast, and the dev overlay replaces
		// the page (REQ-ACT-10, REQ-DEV-06). The status stays 200 so
		// the adapter stream reaches the browser; res.Err carries the
		// error for the dev overlay.
		ctx.res.Patches = append(ctx.res.Patches, ToastPatch{Text: handlerErr.Error(), Kind: ToastError})
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
