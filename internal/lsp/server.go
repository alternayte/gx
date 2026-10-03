package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"sort"
	"sync"

	"github.com/alternayte/gx/internal/compiler"
)

// Options configure one server process.
type Options struct {
	// Root is the module root. initialize can supply it instead through
	// rootUri or workspaceFolders.
	Root string
	// Log receives server log output, or nil.
	Log io.Writer
}

// Server is one gx language server.
type Server struct {
	mu        sync.Mutex // documents and publish state
	diagMu    sync.Mutex // one diagnose pass at a time
	wmu       sync.Mutex // response and notification writes
	root      string
	session   *compiler.Session
	docs      map[string]*document
	published map[string]bool
	out       io.Writer
	log       io.Writer
	exiting   bool
	done      chan struct{}
}

// Serve runs the server loop until exit or EOF.
func Serve(r io.Reader, w io.Writer, opts Options) error {
	s := &Server{
		root:      opts.Root,
		session:   compiler.NewSession(),
		docs:      map[string]*document{},
		published: map[string]bool{},
		out:       w,
		log:       opts.Log,
		done:      make(chan struct{}),
	}
	defer close(s.done)
	go s.watch()
	br := bufio.NewReader(r)
	for {
		msg, err := readMessage(br)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := s.handle(msg); err != nil {
			return err
		}
		if s.exiting {
			return nil
		}
	}
}

// handle dispatches one message.
func (s *Server) handle(msg *message) error {
	switch {
	case msg.Method != "" && msg.ID != nil:
		return s.handleRequest(msg)
	case msg.Method != "":
		return s.handleNotification(msg)
	default:
		return nil // a response to a server request: none yet
	}
}

func (s *Server) handleRequest(msg *message) error {
	var params json.RawMessage = msg.Params
	switch msg.Method {
	case "initialize":
		var p struct {
			RootURI          string `json:"rootUri"`
			WorkspaceFolders []struct {
				URI string `json:"uri"`
			} `json:"workspaceFolders"`
		}
		_ = json.Unmarshal(params, &p)
		s.mu.Lock()
		if s.root == "" {
			switch {
			case p.RootURI != "":
				s.root = uriToPath(p.RootURI)
			case len(p.WorkspaceFolders) > 0:
				s.root = uriToPath(p.WorkspaceFolders[0].URI)
			}
		}
		s.mu.Unlock()
		return s.reply(msg, map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync": map[string]any{
					"openClose": true,
					"change":    1, // full
					"save":      true,
				},
				"completionProvider": map[string]any{
					"triggerCharacters": []string{"<", ".", "{", "$", ":", " ", "\"", "'", "-", "/"},
				},
				"hoverProvider":              true,
				"definitionProvider":         true,
				"renameProvider":             map[string]any{"prepareProvider": false},
				"documentFormattingProvider": true,
				"codeActionProvider": map[string]any{
					"codeActionKinds": []string{"quickfix", "source.organizeImports"},
				},
				"semanticTokensProvider": map[string]any{
					"legend": map[string]any{
						"tokenTypes":     semanticTokenTypes,
						"tokenModifiers": []string{},
					},
					"full": true,
				},
				"inlayHintProvider": true,
			},
			"serverInfo": map[string]any{"name": "gx", "version": "0.1.0"},
		})
	case "shutdown":
		return s.reply(msg, nil)
	case "textDocument/completion":
		return s.reply(msg, s.completion(params))
	case "textDocument/hover":
		return s.reply(msg, s.hover(params))
	case "textDocument/definition":
		return s.reply(msg, s.definition(params))
	case "textDocument/rename":
		return s.reply(msg, s.rename(params))
	case "textDocument/formatting":
		return s.reply(msg, s.formatting(params))
	case "textDocument/codeAction":
		return s.reply(msg, s.codeAction(params))
	case "textDocument/semanticTokens/full":
		return s.reply(msg, s.semanticTokens(params))
	case "textDocument/inlayHint":
		return s.reply(msg, s.inlayHints(params))
	default:
		return s.replyError(msg, errMethodNotFound, "unknown method "+msg.Method)
	}
}

func (s *Server) handleNotification(msg *message) error {
	switch msg.Method {
	case "initialized":
		return nil
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Text    string `json:"text"`
				Version int    `json:"version"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil
		}
		path := uriToPath(p.TextDocument.URI)
		s.diagMu.Lock()
		s.docs[path] = &document{Path: path, Text: p.TextDocument.Text, Version: p.TextDocument.Version}
		s.session.SetOverlay(path, []byte(p.TextDocument.Text))
		err := s.diagnoseLocked()
		s.diagMu.Unlock()
		return err
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil
		}
		path := uriToPath(p.TextDocument.URI)
		s.diagMu.Lock()
		doc := s.docs[path]
		if doc == nil {
			doc = &document{Path: path}
			s.docs[path] = doc
		}
		if len(p.ContentChanges) > 0 {
			doc.Text = p.ContentChanges[len(p.ContentChanges)-1].Text
		}
		doc.Version = p.TextDocument.Version
		s.session.SetOverlay(path, []byte(doc.Text))
		err := s.diagnoseLocked()
		s.diagMu.Unlock()
		return err
	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil
		}
		path := uriToPath(p.TextDocument.URI)
		s.diagMu.Lock()
		delete(s.docs, path)
		s.session.ClearOverlay(path)
		err := s.diagnoseLocked()
		s.diagMu.Unlock()
		return err
	case "textDocument/didSave":
		return s.diagnose()
	case "workspace/didChangeWatchedFiles":
		return s.diagnose()
	case "exit":
		s.exiting = true
		return nil
	default:
		return nil
	}
}

func (s *Server) write(v any) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return writeMessage(s.out, v)
}

func (s *Server) reply(msg *message, result any) error {
	return s.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"result":  result,
	})
}

func (s *Server) replyError(msg *message, code int, text string) error {
	return s.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"error":   map[string]any{"code": code, "message": text},
	})
}

// publishDiagnostics sends one notification.
type publishDiagnostics struct {
	URI         string          `json:"uri"`
	Version     int             `json:"version,omitempty"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

// lspDiagnostic is one diagnostic in LSP shape.
type lspDiagnostic struct {
	Range    rng    `json:"range"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// diagnose runs the compiler over the module with the open buffers and
// publishes the diagnostics of every open file (REQ-DEV-08).
func (s *Server) diagnose() error {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	return s.diagnoseLocked()
}

// diagnoseLocked runs one check pass. The caller holds diagMu.
func (s *Server) diagnoseLocked() error {
	if s.root == "" {
		return nil
	}
	s.session.Generate(s.root)
	diags := s.session.Diagnostics()
	byFile := map[string][]lspDiagnostic{}
	for _, d := range diags {
		path := d.File
		doc := s.docs[path]
		var r rng
		if doc != nil {
			r = doc.rangeOf(d.Line, d.Col)
		} else {
			r = rng{Start: position{Line: max(0, d.Line-1), Character: max(0, d.Col-1)}}
			r.End = r.Start
		}
		byFile[path] = append(byFile[path], lspDiagnostic{
			Range: r, Severity: 1, Code: d.Code, Source: "gx", Message: d.Msg,
		})
	}
	paths := map[string]bool{}
	for path := range s.docs {
		paths[path] = true
	}
	for path := range s.published {
		paths[path] = true
	}
	for path := range byFile {
		paths[path] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		items := byFile[path]
		if items == nil {
			items = []lspDiagnostic{}
		}
		s.published[path] = len(items) > 0
		version := 0
		if doc := s.docs[path]; doc != nil {
			version = doc.Version
		}
		if err := s.write(map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/publishDiagnostics",
			"params":  publishDiagnostics{URI: pathToURI(path), Version: version, Diagnostics: items},
		}); err != nil {
			return err
		}
	}
	return nil
}

// rangeOf converts a 1-based line and byte column to an LSP range.
func (d *document) rangeOf(line, col int) rng {
	pos := position{Line: max(0, line-1)}
	text := d.line(pos.Line)
	pos.Character = utf16Len(text[:min(len(text), max(0, col-1))])
	end := pos
	end.Character = min(utf16Len(text), pos.Character+1)
	return rng{Start: pos, End: end}
}

// doc returns the open document of a file, or nil.
func (s *Server) doc(uri string) *document {
	return s.docs[uriToPath(uri)]
}
