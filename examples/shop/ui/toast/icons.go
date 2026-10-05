package toast

import "github.com/alternayte/gx"

// The icon bodies come from Lucide (https://lucide.dev), ISC License,
// Copyright (c) Lucide Contributors.
const (
	iconCircleCheck   = `<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>`
	iconInfo          = `<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>`
	iconTriangleAlert = `<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>`
	iconOctagonX      = `<path d="m15 9-6 6"/><path d="M2.586 16.726A2 2 0 0 1 2 15.312V8.688a2 2 0 0 1 .586-1.414l4.688-4.688A2 2 0 0 1 8.688 2h6.624a2 2 0 0 1 1.414.586l4.688 4.688A2 2 0 0 1 22 8.688v6.624a2 2 0 0 1-.586 1.414l-4.688 4.688a2 2 0 0 1-1.414.586H8.688a2 2 0 0 1-1.414-.586z"/><path d="m9 9 6 6"/>`
	iconLoaderCircle  = `<path d="M21 12a9 9 0 1 1-6.219-8.56"/>`
	iconX             = `<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`
)

// icon returns the icon body of the toast kind, or "" for the default kind.
func (p ToastProps) icon() string {
	switch p.Toast.Kind {
	case gx.ToastSuccess:
		return iconCircleCheck
	case gx.ToastInfo:
		return iconInfo
	case gx.ToastWarning:
		return iconTriangleAlert
	case gx.ToastError:
		return iconOctagonX
	case gx.ToastLoading:
		return iconLoaderCircle
	}
	return ""
}

// iconClass returns the classes of the kind icon. The icon sits on the first
// text line, and the loading icon turns.
func (p ToastProps) iconClass() string {
	const base = "mt-0.5 size-4 shrink-0"
	if p.Toast.Kind == gx.ToastLoading {
		return base + " animate-spin motion-reduce:animate-none"
	}
	return base
}

// invoke returns the attribute that invokes the action of the toast. The
// adapter writes it. data-gx-close on the same button closes the toast.
func (p ToastProps) invoke() gx.Attrs {
	return gx.Attrs{gx.Invoke(p.Toast.Action.Method, string(p.Toast.Action.URL), "")}
}

// Render renders one pushed toast. An app passes it as gx.Config.Toast.
func Render(p gx.ToastPatch) gx.Node {
	return Toast(ToastProps{Toast: p})
}
