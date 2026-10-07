package compiler

import (
	"strings"

	"github.com/alternayte/gx/internal/gxconfig"
)

// adapterTraits is what the compiler knows of one hypermedia adapter
// (REQ-ACT-09). The attribute syntax of an adapter is not here: the adapter
// writes its attributes when a node renders (REQ-PLG-04).
type adapterTraits struct {
	// signals reports whether the adapter has client signals and client
	// expressions.
	signals bool
	// mods are the event modifiers that the adapter can express. nil
	// means every modifier of REQ-ACT-08.
	mods map[string]bool
}

// adapters are the adapters of gx.toml. The key is the value of its
// adapter key.
var adapters = map[string]adapterTraits{
	gxconfig.AdapterDatastar: {signals: true},
	gxconfig.AdapterHtmx: {mods: map[string]bool{
		"once": true, "stop": true, "window": true, "debounce": true, "throttle": true,
	}},
}

// adapterOf returns the adapter name of the app at root: the adapter key of
// its gx.toml, or Datastar. gxconfig.Load refuses an unknown name, and the
// commands report that error, so an unreadable file is Datastar here.
func adapterOf(root string) string {
	cfg, err := gxconfig.Load(root)
	if err != nil || cfg.Adapter == "" {
		return gxconfig.AdapterDatastar
	}
	return cfg.Adapter
}

// needsSignals reports why the adapter of the app cannot write one client
// site, or "" when it can. An adapter with no signals writes one kind of
// site: an on: handler that is one route literal, with modifiers that the
// adapter has.
func (r *typesResult) needsSignals(site *clientSite) string {
	traits := adapters[r.adapter]
	if traits.signals {
		return ""
	}
	name := site.attr.Name
	spec, isOn := strings.CutPrefix(name, "on:")
	if !isOn {
		directive, _, _ := strings.Cut(name, ":")
		return "the " + Quoted(directive) + " directive is a client expression"
	}
	if site.block || strings.Contains(site.attr.Value, "$") {
		return "this on: handler holds signal statements"
	}
	_, mods := splitOnSpec(spec)
	for _, mod := range mods {
		label, _ := splitMod(mod)
		if !traits.mods[label] {
			return "it has no " + Quoted("."+label) + " modifier"
		}
	}
	return ""
}

// adapterDiags reports the signals block and each client expression of a
// file under an adapter with no signals (GX4006).
func (r *typesResult) adapterDiags(pr *probe) []Diagnostic {
	if adapters[r.adapter].signals {
		return nil
	}
	var out []Diagnostic
	fix := "use an action that patches a fragment, or set adapter = \"datastar\" in gx.toml"
	if len(pr.file.Signals) > 0 {
		at := pr.file.Signals[0].At
		out = append(out, Diagnostic{
			Code: CodeAdapterSignals, File: pr.file.File, Line: at.Line, Col: at.Col,
			Msg: "the " + r.adapter + " adapter has no signals: this component declares signal " + Quoted(pr.file.Signals[0].Name),
			Fix: fix,
		})
	}
	for _, site := range pr.clients {
		why := r.needsSignals(site)
		if why == "" {
			continue
		}
		at := site.attr.At
		msg := "the " + r.adapter + " adapter has no signals: " + why
		if strings.HasPrefix(why, "it has no ") {
			msg = "the " + r.adapter + " adapter cannot write this event: " + why
		}
		out = append(out, Diagnostic{
			Code: CodeAdapterSignals, File: pr.file.File, Line: at.Line, Col: at.Col,
			Msg: msg,
			Fix: fix,
		})
	}
	return out
}
