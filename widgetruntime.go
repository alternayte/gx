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
	a.mux.Handle("GET /_gx/"+name, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "text/javascript; charset=utf-8")
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(body)
	}))
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
