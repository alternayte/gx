package gx

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// widgetRuntimeJS is the built widget script. `just runtime` rebuilds it from
// runtime/js/widget.ts; `checks/runtime.sh` fails when it is stale.
//
//go:embed runtime/js/widget.js
var widgetRuntimeJS []byte

// widgetMorphJS is Idiomorph, the morph of a widget. It is the file that the
// htmx adapter pins by its SHA-256.
//
//go:embed adapters/htmx/idiomorph.js
var widgetMorphJS []byte

// widgetBehaviorsJS is the built bundle of the behaviour modules for a
// widget (REQ-ISL-20): the code of behavior.js, tabs.js, toast.js and
// overlay.js, for a shadow root.
//
//go:embed runtime/js/widget-behaviors.js
var widgetBehaviorsJS []byte

// widgetBehaviors returns the name of the behaviour bundle below /_gx/. The
// name holds a hash of the content.
var widgetBehaviors = sync.OnceValue(func() string {
	sum := sha256.Sum256(widgetBehaviorsJS)
	return "widget-behaviors." + hex.EncodeToString(sum[:6]) + ".js"
})

var widgetScriptState struct {
	once sync.Once
	name string
	body []byte
}

// widgetScript returns the widget script of this build and its file name
// below /_gx/. The name holds a hash of the content, so a host page never
// runs the script of a different build (REQ-ISL-19).
func widgetScript() (name string, body []byte) {
	s := &widgetScriptState
	s.once.Do(func() {
		s.body = make([]byte, 0, len(widgetMorphJS)+len(widgetRuntimeJS)+1)
		s.body = append(s.body, widgetMorphJS...)
		s.body = append(s.body, '\n')
		s.body = append(s.body, widgetRuntimeJS...)
		sum := sha256.Sum256(s.body)
		s.name = "widget." + hex.EncodeToString(sum[:6]) + ".js"
	})
	return s.name, s.body
}

// serveWidgetScript serves the widget script to each origin. The file holds
// no data of a user, and a module script of a different origin loads only
// with this header.
func (a *App) serveWidgetScript() {
	name, body := widgetScript()
	for name, body := range map[string][]byte{name: body, widgetBehaviors(): widgetBehaviorsJS} {
		a.mux.Handle("GET /_gx/"+name, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Type", "text/javascript; charset=utf-8")
			h.Set("Access-Control-Allow-Origin", "*")
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
			_, _ = w.Write(body)
		}))
	}
}

var buildIDState struct {
	once sync.Once
	id   string
}

// buildID names the build of this server: a hash of the program file. Each
// answer to a widget carries it, so an open widget sees a new build of the
// server (REQ-ISL-19).
func buildID() string {
	s := &buildIDState
	s.once.Do(func() {
		h := sha256.New()
		if exe, err := os.Executable(); err == nil {
			if f, err := os.Open(exe); err == nil {
				_, err = io.Copy(h, f)
				_ = f.Close()
				if err == nil {
					s.id = hex.EncodeToString(h.Sum(nil)[:8])
					return
				}
			}
		}
		// The program file cannot be read. The start time is then the
		// name: a new process is a new build for an open widget.
		sum := sha256.Sum256([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
		s.id = hex.EncodeToString(sum[:8])
	})
	return s.id
}

var widgetStyleState struct {
	mu     sync.RWMutex
	sheets map[string]widgetStyle
}

// widgetStyle is the stylesheet of one widget and the name of its file.
type widgetStyle struct {
	name string
	css  []byte
}

// SetWidgetStylesheets installs the stylesheet of each widget, by the tag of
// the widget (REQ-ISL-11). The main of an app calls it with
// gxstyles.Widgets(). The stylesheet of a widget holds only the classes that
// the widget uses, and lives in the shadow root of the element.
func SetWidgetStylesheets(sheets map[string][]byte) {
	next := make(map[string]widgetStyle, len(sheets))
	for tag, css := range sheets {
		sum := sha256.Sum256(css)
		next[tag] = widgetStyle{name: tag + "." + hex.EncodeToString(sum[:6]) + ".css", css: append([]byte(nil), css...)}
	}
	widgetStyleState.mu.Lock()
	defer widgetStyleState.mu.Unlock()
	widgetStyleState.sheets = next
}

// widgetStylePath returns the path of the stylesheet of a widget, or "".
func widgetStylePath(tag string) string {
	widgetStyleState.mu.RLock()
	defer widgetStyleState.mu.RUnlock()
	style, ok := widgetStyleState.sheets[tag]
	if !ok {
		return ""
	}
	return BasePath() + "/_gx/widgets/" + style.name
}

// serveWidgetStyles serves the stylesheet of each widget to each origin. The
// name holds a hash of the content.
func (a *App) serveWidgetStyles() {
	a.mux.Handle("GET /_gx/widgets/{name}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		widgetStyleState.mu.RLock()
		var css []byte
		for _, style := range widgetStyleState.sheets {
			if style.name == name {
				css = style.css
				break
			}
		}
		widgetStyleState.mu.RUnlock()
		if css == nil {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/css; charset=utf-8")
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(css)
	}))
}
