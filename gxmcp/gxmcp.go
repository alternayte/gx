// Package gxmcp serves the tools of a Gx app over MCP (REQ-AI-07). It is a
// package of its own because it uses the official MCP Go SDK: an app that
// does not import it does not compile the SDK.
package gxmcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/alternayte/gx"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// callerKey is the context key of the MCP request of a tool call.
type callerKey struct{}

// endpoint is the MCP handler as a route of the app.
type endpoint struct {
	http.Handler
	pattern string
}

func (e endpoint) Pattern() string { return e.pattern }

// Mount serves the tools of the app over MCP streamable HTTP at path:
//
//	gxmcp.Mount(app, "/mcp", auth.Required)
//
// A tool is an action or a form with Tool; no other route is a tool. The
// middleware runs before each MCP request: put the auth of the app there. A
// tool call runs the request of its action through the app with the headers
// of the MCP request. The middleware of the group of the action and the
// cross-origin check then apply to the call as to a user request.
//
// Mount the routes of the app before the first MCP request.
func Mount(app *gx.App, path string, middleware ...func(http.Handler) http.Handler) {
	var once sync.Once
	var server *mcp.Server
	build := func() {
		server = mcp.NewServer(&mcp.Implementation{Name: "gx-app", Version: "1"}, nil)
		for _, info := range app.Tools() {
			name := info.Name
			// A stateless MCP server cannot ask the user. The annotation
			// tells the client that a tool with gx.Confirm changes what
			// the user cannot get back, so the client asks.
			destructive := info.Confirm
			server.AddTool(&mcp.Tool{
				Name:        name,
				Description: info.Description,
				InputSchema: json.RawMessage(info.Schema),
				Annotations: &mcp.ToolAnnotations{ReadOnlyHint: info.ReadOnly, DestructiveHint: &destructive},
			}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				if req.Params == nil {
					return nil, errors.New("gxmcp: a tool call with no params")
				}
				var answer gx.ToolAnswer
				if caller, ok := ctx.Value(callerKey{}).(*http.Request); ok {
					// The request of the action is the request of the
					// caller: host, client address and TLS state too.
					answer = app.CallToolFor(caller.WithContext(ctx), name, req.Params.Arguments)
				} else {
					var header http.Header
					if req.Extra != nil {
						header = req.Extra.Header
					}
					answer = app.CallTool(ctx, header, name, req.Params.Arguments)
				}
				res := &mcp.CallToolResult{IsError: answer.IsError, Content: []mcp.Content{&mcp.TextContent{Text: answer.Text}}}
				if answer.Structured != nil {
					res.StructuredContent = answer.Structured
				}
				return res, nil
			})
		}
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		// The app has its routes when the first request comes.
		once.Do(build)
		return server
	}, &mcp.StreamableHTTPOptions{
		// Each request stands alone: a tool keeps no state of a session.
		Stateless:    true,
		JSONResponse: true,
		// The Host check of the SDK is for a server on a loopback address
		// with no auth. The auth of this endpoint is the middleware, and
		// the app makes the cross-origin check of each request.
		DisableLocalhostProtection: true,
	})
	parts := make([]any, 0, len(middleware)+1)
	for _, mw := range middleware {
		parts = append(parts, mw)
	}
	// Each tool call of an MCP request gets that request.
	withCaller := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, r)))
	})
	parts = append(parts, gx.Handler(endpoint{Handler: withCaller, pattern: "/" + strings.Trim(path, "/")}))
	app.Group("/", parts...)
}
