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
)

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
