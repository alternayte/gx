package lsp_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/lsp"
)

// client speaks base LSP to a server over an in-memory pipe.
type client struct {
	t     *testing.T
	w     io.Writer
	r     *bufio.Reader
	mu    sync.Mutex
	next  int
	pends map[int]chan *rpcMessage
	notes chan *rpcMessage
	done  chan struct{}
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newClient(t *testing.T, root string) *client {
	t.Helper()
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()
	c := &client{
		t:     t,
		w:     cw,
		r:     bufio.NewReader(cr),
		pends: map[int]chan *rpcMessage{},
		notes: make(chan *rpcMessage, 64),
		done:  make(chan struct{}),
	}
	go func() {
		_ = lsp.Serve(sr, sw, lsp.Options{Root: root})
		_ = sw.Close()
	}()
	go c.readLoop()
	t.Cleanup(func() { _ = cw.Close(); _ = cr.Close() })
	return c
}

func (c *client) readLoop() {
	defer close(c.done)
	for {
		msg, err := readMessage(c.r)
		if err != nil {
			return
		}
		if msg.ID != nil && msg.Method == "" {
			var id int
			_ = json.Unmarshal(msg.ID, &id)
			c.mu.Lock()
			ch := c.pends[id]
			delete(c.pends, id)
			c.mu.Unlock()
			if ch != nil {
				ch <- msg
			}
			continue
		}
		if msg.Method != "" && msg.ID == nil {
			select {
			case c.notes <- msg:
			default:
			}
		}
	}
}

func readMessage(r *bufio.Reader) (*rpcMessage, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if after, ok := strings.CutPrefix(line, "Content-Length:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(after))
			if err != nil {
				return nil, err
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("no Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	var msg rpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (c *client) send(v any) {
	c.t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		c.t.Fatal(err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(c.w, header); err != nil {
		c.t.Fatal(err)
	}
	if _, err := c.w.Write(data); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) request(method string, params any) *rpcMessage {
	c.t.Helper()
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan *rpcMessage, 1)
	c.pends[id] = ch
	c.mu.Unlock()
	c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	select {
	case msg := <-ch:
		if msg.Error != nil {
			c.t.Fatalf("%s: %s (%d)", method, msg.Error.Message, msg.Error.Code)
		}
		return msg
	case <-time.After(20 * time.Second):
		c.t.Fatalf("%s: no response", method)
	}
	return nil
}

func (c *client) notify(method string, params any) {
	c.t.Helper()
	c.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *client) waitNotification(method string) *rpcMessage {
	c.t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case msg := <-c.notes:
			if msg.Method == method {
				return msg
			}
		case <-deadline:
			c.t.Fatalf("%s: no notification", method)
		}
	}
}

func (c *client) initialize() {
	c.t.Helper()
	c.request("initialize", map[string]any{
		"processId": nil,
		"rootUri":   nil,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{},
			},
		},
	})
	c.notify("initialized", map[string]any{})
}

func (c *client) didOpen(path, text string) {
	c.t.Helper()
	c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        fileURI(path),
			"languageId": "gx",
			"version":    1,
			"text":       text,
		},
	})
}

func (c *client) didChange(path, text string) {
	c.t.Helper()
	c.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": fileURI(path), "version": 2},
		"contentChanges": []map[string]any{{"text": text}},
	})
}

func (c *client) didSave(path string) {
	c.t.Helper()
	c.notify("textDocument/didSave", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(path)},
	})
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	slash := filepath.ToSlash(abs)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return "file://" + slash
}

// diagnostics decodes one publishDiagnostics notification.
type diagnostics struct {
	URI   string `json:"uri"`
	Items []struct {
		Range struct {
			Start struct{ Line, Character int } `json:"start"`
			End   struct{ Line, Character int } `json:"end"`
		} `json:"range"`
		Severity int    `json:"severity"`
		Code     string `json:"code"`
		Source   string `json:"source"`
		Message  string `json:"message"`
	} `json:"diagnostics"`
}

func (c *client) waitDiagnostics(path string) *diagnostics {
	c.t.Helper()
	uri := fileURI(path)
	for {
		msg := c.waitNotification("textDocument/publishDiagnostics")
		var d diagnostics
		if err := json.Unmarshal(msg.Params, &d); err != nil {
			c.t.Fatal(err)
		}
		if d.URI == uri {
			return &d
		}
	}
}

// positions returns line/character (0-based) of the first occurrence of
// needle in text.
func position(text, needle string) (int, int) {
	i := strings.Index(text, needle)
	if i < 0 {
		panic("needle not found: " + needle)
	}
	line := strings.Count(text[:i], "\n")
	col := i - (strings.LastIndex(text[:i], "\n") + 1)
	return line, col
}

func positionAt(text string, offset int) (int, int) {
	line := strings.Count(text[:offset], "\n")
	col := offset - (strings.LastIndex(text[:offset], "\n") + 1)
	return line, col
}

// module writes a gx test module and returns its root.
func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files["go.mod"] = "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readBody(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

var _ = bytes.MinRead
