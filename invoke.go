package gx

import "strings"

// An invocation is the client call of one action. Its syntax belongs to the
// adapter (REQ-PLG-04), and a component has no request when it builds its
// nodes. Invoke therefore returns a placeholder, and the render pass puts
// the answer of the adapter in its place.
const (
	// invokeKey is the key of a placeholder attribute.
	invokeKey = "\x00gx-invoke"
	// invokeOpen starts a placeholder in an attribute value. The method,
	// the URL and the scope follow, with invokeSep between them and
	// invokeClose at the end.
	invokeOpen  = "\x00gx-invoke\x1f"
	invokeSep   = "\x1f"
	invokeClose = "\x00"
	// onKey is the key of the placeholder attribute of On. Its value
	// holds the fields of the Invocation with invokeSep between them.
	onKey = "\x00gx-on"
	// modSep is between two modifiers of an On placeholder.
	modSep = "\x1e"
)

// Invocation is one client call of an action on an event (REQ-ACT-02,
// REQ-ACT-08).
type Invocation struct {
	// Method is the HTTP method and URL is the address of the action.
	Method string
	URL    string
	// Scope names the invoking component instance, or is empty
	// (REQ-ACT-03).
	Scope string
	// Event is the name of a DOM event, or "load", "visible" or
	// "interval".
	Event string
	// Every is the period of an interval event, for example "5s".
	Every string
	// Mods are the event modifiers in source order.
	Mods []Modifier
	// Optimistic is true when the element changes signals before the
	// request (REQ-ACT-18). The runtime puts the signals back at the
	// first failure, so the adapter must not send the request again.
	Optimistic bool
}

// Modifier is one event modifier of an Invocation. Value is the text in
// its parentheses, for example "300ms" for debounce(300ms).
type Modifier struct {
	Name  string
	Value string
}

// On returns the attribute that invokes an action on an event
// (REQ-ACT-02, REQ-ACT-08). Generated code calls it for an on: handler
// that is one route literal. spec is the text after "on:", for example
// "click.debounce(300ms)". The adapter of the request, or the process
// default, writes its own attributes when the node renders (REQ-PLG-04).
func On(spec, method, url, scope string) Attr {
	return Attr{
		Key:   onKey,
		Value: invokeField(method) + invokeSep + invokeField(url) + invokeSep + invokeField(scope) + invokeSep + invokeField(spec),
	}
}

// ParseOn splits the text after "on:" into an event, the period of an
// interval event and the modifiers. A dot inside parentheses does not
// start a modifier.
func ParseOn(spec string) (event, every string, mods []Modifier) {
	parts := splitOn(spec)
	event = parts[0]
	if name, value, ok := strings.Cut(event, "("); ok {
		event, every = name, strings.TrimSuffix(value, ")")
	}
	for _, part := range parts[1:] {
		name, value, ok := strings.Cut(part, "(")
		if ok {
			value = strings.TrimSuffix(value, ")")
		}
		mods = append(mods, Modifier{Name: name, Value: value})
	}
	return event, every, mods
}

// splitOn splits spec at each dot outside parentheses.
func splitOn(spec string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(spec); i++ {
		switch spec[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '.':
			if depth == 0 {
				parts = append(parts, spec[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, spec[start:])
}

// resolveOn returns the attributes of the adapter for one On placeholder.
// With no adapter the element gets no attribute.
func (st *renderState) resolveOn(value string) []Attr {
	adapter := st.invokeAdapter()
	fields := strings.Split(value, invokeSep)
	if adapter == nil || len(fields) != 4 {
		return nil
	}
	inv := Invocation{Method: fields[0], URL: externalURL(fields[0], fields[1]), Scope: fields[2]}
	inv.Event, inv.Every, inv.Mods = ParseOn(fields[3])
	inv.Mods, inv.Optimistic = cutOptimistic(inv.Mods)
	return adapter.On(inv)
}

// cutOptimistic removes the mark of an optimistic update from the modifiers
// of an On placeholder. The compiler writes the mark; it is not a modifier
// of the event.
func cutOptimistic(mods []Modifier) ([]Modifier, bool) {
	for i, mod := range mods {
		if mod.Name == "optimistic" {
			return append(mods[:i:i], mods[i+1:]...), true
		}
	}
	return mods, false
}

// Invoke returns the attribute that invokes the action at url with the HTTP
// method (REQ-ACT-02). A non-empty scope names the invoking component
// instance (REQ-ACT-03). Generated code puts the Value in the attribute of
// an on: handler. The adapter of the request, or the process default,
// writes the attribute when the node renders.
func Invoke(method, url, scope string) Attr {
	return Attr{
		Key:   invokeKey,
		Value: invokeOpen + invokeField(method) + invokeSep + invokeField(url) + invokeSep + invokeField(scope) + invokeClose,
	}
}

// invokeField removes the placeholder bytes from one field, so a field
// cannot end the placeholder early.
func invokeField(s string) string {
	if strings.IndexByte(s, 0) < 0 && strings.IndexByte(s, 0x1f) < 0 {
		return s
	}
	return strings.NewReplacer("\x00", "", "\x1f", "").Replace(s)
}

// invokeAdapter returns the adapter of the render: the adapter of the
// request, or the process default.
func (st *renderState) invokeAdapter() Adapter {
	if st == nil {
		return AdapterOf(nil)
	}
	if !st.adapterSet {
		st.adapter, st.adapterSet = AdapterOf(st.request), true
	}
	return st.adapter
}

// resolveInvoke replaces every placeholder of one attribute with the
// invocation of the adapter. A placeholder attribute takes the key of the
// adapter too. With no adapter, or a method the adapter cannot invoke, the
// invocation is empty and a placeholder attribute has no key.
func (st *renderState) resolveInvoke(key, value string) (string, string) {
	adapter := st.invokeAdapter()
	var b strings.Builder
	rest := value
	for {
		i := strings.Index(rest, invokeOpen)
		if i < 0 {
			break
		}
		body := rest[i+len(invokeOpen):]
		end := strings.Index(body, invokeClose)
		if end < 0 {
			break
		}
		fields := strings.Split(body[:end], invokeSep)
		if len(fields) != 3 {
			break
		}
		var attr Attr
		if adapter != nil {
			attr = adapter.Invoke(fields[0], externalURL(fields[0], fields[1]), fields[2])
		}
		if key == invokeKey {
			key = attr.Key
		}
		b.WriteString(rest[:i])
		b.WriteString(attr.Value)
		rest = body[end+len(invokeClose):]
	}
	if key == invokeKey {
		key = ""
	}
	b.WriteString(rest)
	return key, b.String()
}
