package compiler

import (
	"strconv"
	"strings"
)

// sharedSignals returns the shared signals of a file, by their lower-first
// names in the order of the block (REQ-ACT-21).
func sharedSignals(f *File) []string {
	var out []string
	for _, s := range f.Signals {
		if s.Shared {
			out = append(out, lowerFirst(s.Name))
		}
	}
	return out
}

// roomProp returns the name of the prop of type gx.Room, or "".
func roomProp(f *File) string {
	for _, p := range f.Props {
		if strings.TrimSpace(p.Type) == "gx.Room" {
			return p.Name
		}
	}
	return ""
}

// sharedAttrs returns the attributes of the root of a component with shared
// signals: the signed room key, the address of the routes of the room, and
// the effect that gives each change of a shared signal to the runtime.
func (g *gen) sharedAttrs() []string {
	names := sharedSignals(g.file)
	if len(names) == 0 {
		return nil
	}
	room := roomProp(g.file)
	if room == "" {
		at := g.file.Signals[0]
		for _, s := range g.file.Signals {
			if s.Shared {
				at = s
				break
			}
		}
		g.diags = append(g.diags, Diagnostic{
			Code: CodeSharedSignal, File: g.file.File, Line: at.At.Line, Col: at.At.Col,
			Msg: "the shared signal " + Quoted(at.Name) + " needs a room: the component has no prop of type gx.Room",
			Fix: "add the prop Room gx.Room, and give it gx.RoomKey(c, key) from a loader",
		})
		return nil
	}
	base := strconv.Quote(g.file.Package + "." + g.name)
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = strconv.Quote(name)
	}
	return []string{
		"gx.Attr{Key: \"data-gx-room\", Value: string(p." + room + "), Kind: gx.AttrText}",
		"gx.Attr{Key: \"data-gx-room-url\", Value: gx.RoomURL(" + base + "), Kind: gx.AttrURL}",
		"gx.Attr{Key: \"data-effect\", Value: gx.ShareEffect(" + base + ", p.GxKey, " + strings.Join(quoted, ", ") + "), Kind: gx.AttrText}",
	}
}

// emitShared writes the declarations of a component with shared signals:
// the routes of its room, which the app mounts in a group, and the names of
// the fields of its Signals struct for the rules of a write (SI-17).
func (g *gen) emitShared() {
	names := sharedSignals(g.file)
	if len(names) == 0 || roomProp(g.file) == "" {
		return
	}
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = strconv.Quote(name)
	}
	g.write("")
	g.write("// %sRoom holds the routes of the shared signals of %s (REQ-ACT-21).", g.name, g.name)
	g.write("// Mount it in a group of the app: each viewer of a room then passes the")
	g.write("// middleware of that group.")
	g.write("var %sRoom = gx.SharedSignals[%sSignals](%s, %s)", g.name, g.name, strconv.Quote(g.file.Package+"."+g.name), strings.Join(quoted, ", "))
	g.write("")
	g.write("// GxFieldName names the signal of a field for the rules of %sSignals.", g.name)
	g.write("func (s *%sSignals) GxFieldName(ptr any) string {", g.name)
	g.ind++
	g.write("switch ptr {")
	for _, s := range g.file.Signals {
		g.write("case &s.%s:", s.Name)
		g.ind++
		g.write("return %s", strconv.Quote(lowerFirst(s.Name)))
		g.ind--
	}
	g.write("}")
	g.write("return \"\"")
	g.ind--
	g.write("}")
}
