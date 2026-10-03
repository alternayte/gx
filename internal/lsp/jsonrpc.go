// Package lsp implements the gx language server over base LSP on stdio
// (REQ-DEV-08, REQ-TLS-04). It speaks standard LSP with no editor-specific
// extensions.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// message is one JSON-RPC 2.0 message.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

// responseError is the error object of a failed request.
type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// protocol errors of the LSP base protocol.
const (
	errParse          = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

// readMessage reads one Content-Length framed message.
func readMessage(r *bufio.Reader) (*message, error) {
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
				return nil, fmt.Errorf("lsp: bad Content-Length: %w", err)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("lsp: message without Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("lsp: bad message: %w", err)
	}
	return &m, nil
}

// writeMessage writes one Content-Length framed message.
func writeMessage(w io.Writer, m any) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
