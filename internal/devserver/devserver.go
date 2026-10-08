// Package devserver is the `gx dev` loop: build, run, watch, reload
// (REQ-DEV-01, REQ-DEV-03, REQ-DEV-06, REQ-DEV-07). It is repo tooling and
// never ships in a production binary.
package devserver

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/gxstyles"
	"github.com/alternayte/gx/internal/islands"
)

//go:embed devclient.js
var devClientJS []byte

// Options configures the dev loop.
type Options struct {
	// Dir is the app module directory.
	Dir string
	// Main is the main package to build, relative to Dir. Default
	// "./cmd/app", or the only "./cmd/*" directory that holds a main.
	Main string
	// Addr is the address the dev proxy listens on.
	Addr string
	// Log receives the build and run output.
	Log io.Writer
}

// Overlay is one compile, type or render error for the browser overlay
// (REQ-DEV-06).
type Overlay struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
	Col   int    `json:"col,omitempty"`
	Link  string `json:"link,omitempty"`
}

// Run runs the dev loop until ctx ends.
func Run(ctx context.Context, opt Options) error {
	if opt.Dir == "" {
		opt.Dir = "."
	}
	if opt.Addr == "" {
		opt.Addr = "127.0.0.1:3333"
	}
	if opt.Log == nil {
		opt.Log = os.Stdout
	}
	dir, err := filepath.Abs(opt.Dir)
	if err != nil {
		return err
	}
	if opt.Main == "" {
		opt.Main, err = DetectMain(dir)
		if err != nil {
			return err
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	s := &server{opt: opt, dir: dir, session: compiler.NewSession(), clients: map[chan []byte]bool{},
		secret: hex.EncodeToString(secret)}
	return s.run(ctx)
}

// DetectMain finds the app main package when none is given.
func DetectMain(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "cmd", "app", "main.go")); err == nil {
		return "./cmd/app", nil
	}
	entries, err := os.ReadDir(filepath.Join(dir, "cmd"))
	if err != nil {
		return "", errors.New("gx dev: no -main given and no cmd/ directory found")
	}
	var found []string
	for _, e := range entries {
		// cmd/gx is the project's own Gx command line tool, not the app.
		if !e.IsDir() || e.Name() == "gx" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "cmd", e.Name(), "main.go")); err == nil {
			found = append(found, "./cmd/"+e.Name())
		}
	}
	switch len(found) {
	case 0:
		return "", errors.New("gx dev: no -main given and no cmd/*/main.go found")
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("gx dev: several mains found (%s); pass -main", strings.Join(found, ", "))
	}
}

type server struct {
	opt     Options
	dir     string
	session *compiler.Session

	appPort int
	appCmd  *exec.Cmd
	// appParent is the write end of the pipe that tells the app that gx
	// dev is alive.
	appParent *os.File
	// secret is the secret of this run between gx dev and the app.
	secret string

	mu      sync.Mutex
	clients map[chan []byte]bool
	lastErr *Overlay
	snap    map[string]time.Time

	// sigs holds the signature of each .gx file at the last build, and
	// classes the class list of that build. A change that keeps both can
	// swap into the running app (REQ-DEV-02).
	sigs map[string]string
	// generated holds the generated Go files of the last build.
	generated map[string][]byte
	// buildFailed is true while the last build of the app failed.
	buildFailed bool
	classes     []byte

	// searchMu guards the on-demand Pagefind index (REQ-CNT-07).
	searchMu   sync.Mutex
	searchSnap map[string]time.Time
}

func (s *server) run(ctx context.Context) error {
	s.appPort = freePort()
	bin := execname.Name(filepath.Join(s.workDir(), "app"))
	if err := os.MkdirAll(s.workDir(), 0o755); err != nil {
		return err
	}
	if ok, ov := s.build(ctx, bin); !ok {
		s.buildFailed = true
		s.setErr(ov)
		fmt.Fprintf(s.opt.Log, "gx dev: %s: %s\n", ov.Title, ov.Text)
	}
	if err := s.startApp(ctx, bin); err != nil {
		return err
	}
	defer s.stopApp()
	s.waitReady(ctx)
	go s.watch(ctx, bin)

	appURL := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(s.appPort))}
	proxy := httputil.NewSingleHostReverseProxy(appURL)
	proxy.ModifyResponse = s.injectDevClient

	mux := http.NewServeMux()
	mux.HandleFunc("GET /_gx/dev", s.serveEvents)
	mux.HandleFunc("GET /_gx/dev-client.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(devClientJS)
	})
	mux.HandleFunc("GET /pagefind/", s.serveSearch)
	// The dev host pages of the widgets (REQ-ISL-23).
	mux.HandleFunc("GET /_gx/widgets", s.serveWidgets(proxy))
	mux.HandleFunc("GET /_gx/widgets/{name}", s.serveWidgets(proxy))
	mux.Handle("/", proxy)

	srv := &http.Server{Addr: s.opt.Addr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	fmt.Fprintf(s.opt.Log, "gx dev: http://%s (app %s)\n", s.opt.Addr, appURL)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *server) workDir() string { return filepath.Join(s.dir, ".gx", "dev") }

// build runs gx generate and go build. It returns the overlay on failure
// (REQ-DEV-03, REQ-DEV-06).
func (s *server) build(ctx context.Context, bin string) (bool, *Overlay) {
	if files, diags := s.session.Generate(s.dir); len(diags) > 0 {
		return false, diagsOverlay(diags)
	} else {
		for path, src := range files {
			// A file that did not change keeps its time: the props file of
			// an island is a .ts file, and the watcher follows .ts files.
			if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, src) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return false, &Overlay{Title: "generate failed", Text: err.Error()}
			}
			if err := os.WriteFile(path, src, 0o644); err != nil {
				return false, &Overlay{Title: "generate failed", Text: err.Error()}
			}
		}
		s.remember(files)
		if _, err := gxstyles.Build(ctx, s.dir, false); err != nil {
			return false, &Overlay{Title: "styles failed", Text: err.Error()}
		}
		// The dev bundle keeps names and has source maps (REQ-ISL-03).
		if _, err := islands.Write(s.dir, islands.Options{}); err != nil {
			return false, islandsOverlay(err)
		}
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", "gxdev", "-o", bin, s.opt.Main)
	cmd.Dir = s.dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, buildOverlay(string(out), s.dir)
	}
	return true, nil
}

// parentPipeEnv names the file descriptor of the parent pipe in the app. The
// gxdev build of package gx reads the same name.
const parentPipeEnv = "GX_DEV_PARENT_FD"

// The secret between gx dev and the app. The app runs the code of a swap
// request, so it takes one only with the secret of this run. The gxdev build
// of package gx reads the same names.
const (
	devSecretEnv    = "GX_DEV_SECRET"
	devSecretHeader = "Gx-Dev-Secret"
)

// startApp runs the built binary. It reads GX_DEV_ADDR for its listen
// address (REQ-DEV-01).
func (s *server) startApp(ctx context.Context, bin string) error {
	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = s.dir
	cmd.Env = append(os.Environ(),
		"GX_DEV_ADDR="+net.JoinHostPort("127.0.0.1", strconv.Itoa(s.appPort)),
		"GX_DEV=1",
		devSecretEnv+"="+s.secret,
	)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = s.opt.Log
	// The app holds the read end of a pipe and gx dev holds the write end.
	// When gx dev ends for any reason, also a kill it cannot catch, the
	// write end closes and the app exits (dev_gxdev.go in package gx). Windows has no
	// ExtraFiles; there the signal path is the only one.
	var parent *os.File
	if runtime.GOOS != "windows" {
		r, w, err := os.Pipe()
		if err != nil {
			return err
		}
		defer r.Close()
		parent = w
		cmd.ExtraFiles = []*os.File{r}
		cmd.Env = append(cmd.Env, parentPipeEnv+"=3")
	}
	if err := cmd.Start(); err != nil {
		if parent != nil {
			_ = parent.Close()
		}
		return err
	}
	s.appCmd = cmd
	s.appParent = parent
	go s.captureErrors(stderr)
	return nil
}

// captureErrors shows a render panic or a fatal log line in the overlay
// (REQ-DEV-06).
func (s *server) captureErrors(r io.Reader) {
	sc := bufio.NewScanner(r)
	var lines []string
	for sc.Scan() {
		line := sc.Text()
		lines = append(lines, line)
		if len(lines) > 40 {
			lines = lines[1:]
		}
		fmt.Fprintln(s.opt.Log, line)
		if strings.Contains(line, "panic:") || strings.Contains(line, "fatal error:") {
			text := strings.Join(lines, "\n")
			ov := &Overlay{Title: "app error", Text: text}
			if file, ln, col, ok := firstPosition(text); ok {
				ov.File, ov.Line, ov.Col = file, ln, col
				ov.Link = editorLink(s.dir, file, ln, col)
			}
			s.setErr(ov)
			s.broadcast("overlay", ov)
		}
	}
}

func (s *server) stopApp() {
	if s.appCmd != nil && s.appCmd.Process != nil {
		_ = s.appCmd.Process.Kill()
		_, _ = s.appCmd.Process.Wait()
		s.appCmd = nil
	}
	if s.appParent != nil {
		_ = s.appParent.Close()
		s.appParent = nil
	}
}

// waitReady blocks until the app answers (REQ-DEV-03).
func (s *server) waitReady(ctx context.Context) {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.appPort)), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// injectDevClient adds the dev client module to every HTML page
// (REQ-DEV-01).
func (s *server) injectDevClient(res *http.Response) error {
	if res.StatusCode != http.StatusOK || !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		return nil
	}
	body, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		return err
	}
	script := `<script type="module" src="/_gx/dev-client.js"></script>`
	if m := policyNonce.FindStringSubmatch(res.Header.Get("Content-Security-Policy")); m != nil {
		// The app sent a nonce policy; the dev client must carry the
		// nonce or the browser blocks it (SI-11).
		script = `<script type="module" src="/_gx/dev-client.js" nonce="` + m[1] + `"></script>`
	}
	marker := []byte("</head>")
	if i := indexFold(body, marker); i >= 0 {
		body = append(body[:i], append([]byte(script), body[i:]...)...)
	} else if i := indexFold(body, []byte("</body>")); i >= 0 {
		body = append(body[:i], append([]byte(script), body[i:]...)...)
	} else {
		body = append(body, script...)
	}
	res.Body = io.NopCloser(strings.NewReader(string(body)))
	res.ContentLength = int64(len(body))
	res.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

// policyNonce finds the nonce source of a Content-Security-Policy.
var policyNonce = regexp.MustCompile(`'nonce-([A-Za-z0-9+/=_-]+)'`)

// indexFold returns the index of the first case-insensitive match of sep.
func indexFold(b, sep []byte) int {
	return strings.Index(strings.ToLower(string(b)), strings.ToLower(string(sep)))
}

func (s *server) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "gx dev: streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 8)
	s.subscribe(ch)
	defer s.unsubscribe(ch)
	flusher.Flush()
	if ov := s.getErr(); ov != nil {
		writeEvent(w, "overlay", ov)
		flusher.Flush()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			_, _ = w.Write(msg)
			flusher.Flush()
		}
	}
}

func (s *server) subscribe(ch chan []byte) {
	s.mu.Lock()
	s.clients[ch] = true
	s.mu.Unlock()
}

func (s *server) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
}

func (s *server) broadcast(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	s.mu.Lock()
	for ch := range s.clients {
		select {
		case ch <- []byte(msg):
		default:
		}
	}
	s.mu.Unlock()
}

func (s *server) setErr(ov *Overlay) {
	s.mu.Lock()
	s.lastErr = ov
	s.mu.Unlock()
}

func (s *server) getErr() *Overlay {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

func writeEvent(w io.Writer, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
}

// watch polls the app tree and rebuilds on a change (REQ-DEV-03).
func (s *server) watch(ctx context.Context, bin string) {
	s.snap = s.snapshot()
	// The loop has two speeds. Every tick it reads the time of each .gx
	// file it knows, which is cheap, so a markup edit starts its swap in a
	// few milliseconds (NFR-01). Every tenth tick it walks the whole tree
	// for every other change, as before.
	ticker := time.NewTicker(fastTick)
	defer ticker.Stop()
	changed := false
	var quiet time.Time
	var edited map[string]time.Time
	for tick := 0; ; tick++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if !changed {
			if edited != nil {
				// One tick after the edit: the editor has closed the file.
				for path, at := range edited {
					s.snap[path] = at
				}
				paths := make([]string, 0, len(edited))
				for path := range edited {
					paths = append(paths, path)
				}
				edited = nil
				if !s.swap(ctx, paths) {
					s.rebuild(ctx, bin)
				}
				continue
			}
			if edited = s.editedTemplates(); edited != nil {
				continue
			}
		}
		if tick%slowEvery != 0 {
			continue
		}
		now := s.snapshot()
		if !sameSnapshot(s.snap, now) {
			// A template can change between the check of the .gx files
			// and this walk. That is still a markup edit, not a reason to
			// rebuild.
			if only := editedOnly(s.snap, now); only != nil && !changed {
				edited = only
				continue
			}
			s.snap = now
			changed = true
			edited = nil
			quiet = time.Now()
		}
		if !changed || time.Since(quiet) < 100*time.Millisecond {
			continue
		}
		changed = false
		s.rebuild(ctx, bin)
	}
}

// fastTick is the period of the check of the .gx files, and slowEvery is
// the count of ticks between two walks of the whole tree (200 ms).
const (
	fastTick  = 20 * time.Millisecond
	slowEvery = 10
)

// editedOnly compares two snapshots. When they hold the same files and
// only .gx files have a new time, it returns those files with the new
// time; otherwise nil.
func editedOnly(old, now map[string]time.Time) map[string]time.Time {
	if len(old) != len(now) {
		return nil
	}
	out := map[string]time.Time{}
	for path, at := range now {
		was, ok := old[path]
		if !ok {
			return nil
		}
		if at.Equal(was) {
			continue
		}
		if !strings.HasSuffix(path, ".gx") {
			return nil
		}
		out[path] = at
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// editedTemplates returns the .gx files of the last snapshot whose time
// changed, with the new time. It returns nil when none changed, or when a
// file is gone: the walk of the tree handles that.
func (s *server) editedTemplates() map[string]time.Time {
	var out map[string]time.Time
	for path, was := range s.snap {
		if !strings.HasSuffix(path, ".gx") {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil
		}
		if !info.ModTime().Equal(was) {
			if out == nil {
				out = map[string]time.Time{}
			}
			out[path] = info.ModTime()
		}
	}
	return out
}

// remember records what the build of the app was made from: the signature
// of each .gx file and the class list.
func (s *server) remember(files map[string][]byte) {
	s.sigs = map[string]string{}
	for path := range s.snapshot() {
		if strings.HasSuffix(path, ".gx") {
			if sig, ok := compiler.FileSignature(path); ok {
				s.sigs[path] = sig
			}
		}
	}
	s.classes = files[compiler.ClassesPath(s.dir)]
	// The generated Go code that the last build wrote. A swap compares
	// against it in memory: a read of each file costs time on every save.
	s.generated = make(map[string][]byte, len(files))
	for path, src := range files {
		if strings.HasSuffix(path, "_gx.go") {
			s.generated[path] = src
		}
	}
}

// swap puts a markup edit into the running app with no rebuild
// (REQ-DEV-02). It reports false when the change needs the rebuild path: a
// changed signature, a new class for the stylesheet, or code that the
// interpreter of the app refuses (REQ-DEV-04). A template with an error is
// handled here: the browser shows the overlay.
func (s *server) swap(ctx context.Context, paths []string) bool {
	start := time.Now()
	if s.buildFailed {
		// The running app is older than the tree, and the error of the
		// build is still in the files. A build shows it or clears it
		// (REQ-DEV-06).
		fmt.Fprintln(s.opt.Log, "gx dev: rebuild: the last build failed")
		return false
	}
	for _, path := range paths {
		sig, ok := compiler.FileSignature(path)
		if !ok {
			continue // a parse error: the generator reports it below
		}
		if was, known := s.sigs[path]; !known || was != sig {
			fmt.Fprintf(s.opt.Log, "gx dev: rebuild: the props, signals, fragments or imports of %s changed\n", filepath.Base(path))
			return false
		}
	}
	files, diags := s.session.Generate(s.dir)
	if len(diags) > 0 {
		ov := diagsOverlay(diags)
		s.setErr(ov)
		s.broadcast("overlay", ov)
		return true
	}
	if !bytes.Equal(files[compiler.ClassesPath(s.dir)], s.classes) {
		fmt.Fprintln(s.opt.Log, "gx dev: rebuild: the stylesheet needs a new class")
		return false
	}
	// A swap changes the code of the edited templates only. When the
	// generator writes different code for any other file, the compiled app
	// is older than the tree: build it (REQ-DEV-03).
	edited := map[string]bool{}
	for _, path := range paths {
		edited[strings.TrimSuffix(path, ".gx")+"_gx.go"] = true
	}
	for generated, src := range files {
		if edited[generated] || !strings.HasSuffix(generated, "_gx.go") {
			continue
		}
		if old, ok := s.generated[generated]; !ok || !bytes.Equal(old, src) {
			fmt.Fprintf(s.opt.Log, "gx dev: rebuild: the generated code of %s changed\n", filepath.Base(generated))
			return false
		}
	}
	for _, path := range paths {
		generated := strings.TrimSuffix(path, ".gx") + "_gx.go"
		src, ok := files[generated]
		if !ok {
			fmt.Fprintf(s.opt.Log, "gx dev: rebuild: no generated code for %s\n", filepath.Base(path))
			return false
		}
		// The generated file on the disk follows the template, so a later
		// build compiles the same code.
		if old, err := os.ReadFile(generated); err != nil || !bytes.Equal(old, src) {
			if err := os.WriteFile(generated, src, 0o644); err != nil {
				return false
			}
		}
		if reason := s.swapFile(ctx, compiler.PackagePath(filepath.Dir(path)), filepath.Base(generated), src); reason != "" {
			fmt.Fprintf(s.opt.Log, "gx dev: rebuild: %s\n", reason)
			return false
		}
	}
	s.setErr(nil)
	s.broadcast("reload", map[string]any{"swap": true, "ms": time.Since(start).Milliseconds()})
	return true
}

// swapFile sends the generated code of one file to the app. It returns ""
// when the app runs the code now, and the reason when it does not.
func (s *server) swapFile(ctx context.Context, pkg, file string, src []byte) string {
	payload, err := json.Marshal(map[string]string{"package": pkg, "file": file, "source": string(src)})
	if err != nil {
		return err.Error()
	}
	target := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(s.appPort)) + "/_gx/dev/swap"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(devSecretHeader, s.secret)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error()
	}
	defer res.Body.Close()
	var answer struct {
		Swapped bool   `json:"swapped"`
		Reason  string `json:"reason"`
	}
	if res.StatusCode == http.StatusOK {
		return ""
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil || answer.Reason == "" {
		return "the app answered " + res.Status + " to the swap"
	}
	return answer.Reason
}

// rebuild generates, builds, restarts and tells the browser to morph
// (REQ-DEV-03).
func (s *server) rebuild(ctx context.Context, bin string) {
	ok, ov := s.build(ctx, bin)
	s.buildFailed = !ok
	if !ok {
		s.setErr(ov)
		s.broadcast("overlay", ov)
		return
	}
	s.stopApp()
	if err := s.startApp(ctx, bin); err != nil {
		s.setErr(&Overlay{Title: "run failed", Text: err.Error()})
		s.broadcast("overlay", s.getErr())
		return
	}
	s.waitReady(ctx)
	s.setErr(nil)
	s.broadcast("reload", map[string]string{})
}

// watchPaths are the file kinds the watcher follows.
func watchFile(path string) bool {
	base := filepath.Base(path)
	// Generated code is the result of a build, not a reason for one: the
	// Go files of the compiler and the props type of an island.
	if strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, ".props.ts") {
		return false
	}
	switch {
	case strings.HasSuffix(path, ".gx"), strings.HasSuffix(path, ".go"),
		strings.HasSuffix(path, ".md"), strings.HasSuffix(path, ".markdown"),
		strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".ts"),
		base == "go.mod", base == "go.sum", base == "gx.toml":
		return true
	}
	return false
}

func (s *server) snapshot() map[string]time.Time {
	out := map[string]time.Time{}
	_ = filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "node_modules", ".gx", "vendor", "testdata":
				return fs.SkipDir
			}
			return nil
		}
		if !watchFile(path) {
			return nil
		}
		if info, err := d.Info(); err == nil {
			out[path] = info.ModTime()
		}
		return nil
	})
	return out
}

func sameSnapshot(a, b map[string]time.Time) bool {
	if len(a) != len(b) {
		return false
	}
	for path, t := range a {
		if other, ok := b[path]; !ok || !other.Equal(t) {
			return false
		}
	}
	return true
}

// freePort returns a free local TCP port.
func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

var positionRe = regexp.MustCompile(`([^\s:]+\.(?:gx|go|ts)):(\d+):(\d+)`)

// firstPosition finds the first file:line:col in a message.
func firstPosition(text string) (string, int, int, bool) {
	m := positionRe.FindStringSubmatch(text)
	if m == nil {
		return "", 0, 0, false
	}
	line, _ := strconv.Atoi(m[2])
	col, _ := strconv.Atoi(m[3])
	return m[1], line, col, true
}

// buildOverlay turns go build output into an overlay (REQ-DEV-06).
func buildOverlay(text, dir string) *Overlay {
	ov := &Overlay{Title: "build failed", Text: strings.TrimSpace(text)}
	if file, line, col, ok := firstPosition(text); ok {
		ov.File, ov.Line, ov.Col = file, line, col
		ov.Text = withExcerpt(text, dir, file, line)
		ov.Link = editorLink(dir, file, line, col)
	}
	return ov
}

// diagsOverlay turns gx diagnostics into an overlay (REQ-DEV-06).
// islandsOverlay shows the first error of a failed island bundle.
func islandsOverlay(err error) *Overlay {
	ov := &Overlay{Title: "islands failed", Text: err.Error()}
	var berr *islands.Error
	if errors.As(err, &berr) && len(berr.Messages) > 0 && berr.Messages[0].File != "" {
		m := berr.Messages[0]
		ov.File, ov.Line, ov.Col = m.File, m.Line, m.Col
		ov.Text = withExcerpt(ov.Text, "", m.File, m.Line)
		ov.Link = editorLink("", m.File, m.Line, m.Col)
	}
	return ov
}

func diagsOverlay(diags []compiler.Diagnostic) *Overlay {
	var b strings.Builder
	for _, d := range diags {
		fmt.Fprintf(&b, "%s:%d:%d: %s %s\n", d.File, d.Line, d.Col, d.Code, d.Msg)
	}
	ov := &Overlay{Title: "check failed", Text: strings.TrimSpace(b.String())}
	if len(diags) > 0 {
		d := diags[0]
		ov.File, ov.Line, ov.Col = d.File, d.Line, d.Col
		ov.Text = withExcerpt(ov.Text, "", d.File, d.Line)
		ov.Link = editorLink("", d.File, d.Line, d.Col)
	}
	return ov
}

// withExcerpt appends the source line of a position.
func withExcerpt(text, dir, file string, line int) string {
	path := file
	if !filepath.IsAbs(path) && dir != "" {
		path = filepath.Join(dir, file)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return text
	}
	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) {
		return text
	}
	return strings.TrimSpace(text) + "\n\n" + strconv.Itoa(line) + " | " + lines[line-1]
}

// editorLink returns a vscode URL for a position (REQ-DEV-06).
func editorLink(dir, file string, line, col int) string {
	path := file
	if !filepath.IsAbs(path) && dir != "" {
		path = filepath.Join(dir, file)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return fmt.Sprintf("vscode://file/%s:%d:%d", filepath.ToSlash(path), line, col)
}
