package compiler

import (
	"encoding/json"
	"go/types"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// The JSON wire of an app (REQ-ACT-20): an OpenAPI file and a TypeScript
// client for each action with .API(). The route types and the schema of
// REQ-AI-09 are the one source.

// APIDir is the directory of the files that gx api writes, under the module
// root.
const APIDir = "api"

// APIFiles returns the files of the JSON wire of the module under root, by
// name: openapi.json and client.ts. It returns no file for a module with no
// action with .API().
func APIFiles(root string) (map[string][]byte, []Diagnostic) {
	_, diags, res, _, _ := generate(root, nil)
	if len(diags) > 0 || res == nil {
		return nil, diags
	}
	return res.apiFiles(), nil
}

// apiOp is one action of the JSON wire.
type apiOp struct {
	def    *routeDef
	method string
	path   string
	// schema is the schema of the arguments, and fields names the part of
	// the request of each top-level argument.
	schema map[string]any
	fields [][2]string
	result map[string]any
}

func (r *typesResult) apiOps() []apiOp {
	var ops []apiOp
	for _, d := range r.routes {
		if d.tool == nil || !d.tool.api {
			continue
		}
		method, pattern, _ := strings.Cut(d.pattern, " ")
		schema, fields, _ := toolSchema(d)
		op := apiOp{def: d, method: method, path: pathVarRe.ReplaceAllString(pattern, "{$1}"), schema: schema, fields: fields}
		switch {
		case d.tool.result != nil:
			op.result = goSchema(d.tool.result, map[types.Type]bool{})
		case d.tool.unknownResult:
			op.result = map[string]any{}
		}
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].path != ops[j].path {
			return ops[i].path < ops[j].path
		}
		return ops[i].method < ops[j].method
	})
	return ops
}

func (r *typesResult) apiFiles() map[string][]byte {
	ops := r.apiOps()
	if len(ops) == 0 {
		return nil
	}
	return map[string][]byte{"openapi.json": openAPI(ops), "client.ts": tsClient(ops)}
}

// goSchema returns the JSON Schema of the JSON form of a Go type.
func goSchema(t types.Type, seen map[types.Type]bool) map[string]any {
	switch u := t.(type) {
	case *types.Pointer:
		return goSchema(u.Elem(), seen)
	case *types.Named:
		switch u.String() {
		case "time.Time":
			return map[string]any{"type": "string", "format": "date-time"}
		case "github.com/alternayte/gx.Secret":
			return map[string]any{"type": "string"}
		}
		if seen[u] {
			// A type that holds itself: any value.
			return map[string]any{}
		}
		seen[u] = true
		defer delete(seen, u)
		return goSchema(u.Underlying(), seen)
	case *types.Alias:
		return goSchema(types.Unalias(u), seen)
	case *types.Basic:
		switch {
		case u.Info()&types.IsBoolean != 0:
			return map[string]any{"type": "boolean"}
		case u.Info()&types.IsInteger != 0:
			return map[string]any{"type": "integer"}
		case u.Info()&types.IsFloat != 0:
			return map[string]any{"type": "number"}
		case u.Info()&types.IsString != 0:
			return map[string]any{"type": "string"}
		}
	case *types.Slice:
		if b, ok := u.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.Byte {
			// encoding/json writes a []byte as base64 text.
			return map[string]any{"type": "string", "contentEncoding": "base64"}
		}
		return map[string]any{"type": "array", "items": goSchema(u.Elem(), seen)}
	case *types.Array:
		return map[string]any{"type": "array", "items": goSchema(u.Elem(), seen)}
	case *types.Map:
		return map[string]any{"type": "object", "additionalProperties": goSchema(u.Elem(), seen)}
	case *types.Struct:
		props := map[string]any{}
		var required []any
		for i := 0; i < u.NumFields(); i++ {
			f := u.Field(i)
			if !f.Exported() {
				continue
			}
			name, opts, _ := strings.Cut(reflect.StructTag(u.Tag(i)).Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name()
			}
			props[name] = goSchema(f.Type(), seen)
			_, pointer := f.Type().(*types.Pointer)
			if !strings.Contains(opts, "omitempty") && !strings.Contains(opts, "omitzero") && !pointer {
				required = append(required, name)
			}
		}
		out := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			out["required"] = required
		}
		return out
	}
	return map[string]any{}
}

// errorsSchema is the schema of the answer of a failed call.
var errorsSchema = map[string]any{
	"type":     "object",
	"required": []any{"errors"},
	"properties": map[string]any{
		"errors": map[string]any{"type": "array", "items": map[string]any{
			"type":     "object",
			"required": []any{"key"},
			"properties": map[string]any{
				"key":     map[string]any{"type": "string"},
				"field":   map[string]any{"type": "string"},
				"message": map[string]any{"type": "string"},
			},
		}},
	},
}

// openAPI writes the OpenAPI 3.1 file of the actions.
func openAPI(ops []apiOp) []byte {
	paths := map[string]any{}
	for _, op := range ops {
		props, _ := op.schema["properties"].(map[string]any)
		required := map[string]bool{}
		if req, ok := op.schema["required"].([]any); ok {
			for _, name := range req {
				required[name.(string)] = true
			}
		}
		var params []any
		body := map[string]any{}
		var bodyRequired []any
		for _, f := range op.fields {
			name, in := f[0], f[1]
			switch {
			case in == "path" || in == "query" || !hasBody(op.method):
				// A method with no body has each argument in the
				// address.
				if in != "path" {
					in = "query"
				}
				p := map[string]any{"name": name, "in": in, "schema": props[name]}
				if in == "path" || required[name] {
					p["required"] = true
				}
				params = append(params, p)
			default:
				body[name] = props[name]
				if required[name] {
					bodyRequired = append(bodyRequired, name)
				}
			}
		}
		operation := map[string]any{"operationId": toolName(op.def)}
		if op.def.tool.doc != "" {
			operation["description"] = op.def.tool.doc
		}
		if len(params) > 0 {
			operation["parameters"] = params
		}
		if len(body) > 0 {
			schema := map[string]any{"type": "object", "additionalProperties": false, "properties": body}
			if len(bodyRequired) > 0 {
				schema["required"] = bodyRequired
			}
			operation["requestBody"] = map[string]any{
				"required": len(bodyRequired) > 0,
				"content":  map[string]any{"application/json": map[string]any{"schema": schema}},
			}
		}
		fail := map[string]any{
			"description": "The call failed.",
			"content":     map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Errors"}}},
		}
		responses := map[string]any{
			"422":     map[string]any{"description": "A rule of the input failed.", "content": fail["content"]},
			"default": fail,
		}
		if op.result != nil {
			responses["200"] = map[string]any{
				"description": "The result of the action.",
				"content":     map[string]any{"application/json": map[string]any{"schema": op.result}},
			}
		}
		responses["204"] = map[string]any{"description": "The action ran and has no result."}
		operation["responses"] = responses
		item, _ := paths[op.path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[op.path] = item
		}
		item[strings.ToLower(op.method)] = operation
	}
	title := "API"
	if len(ops) > 0 && ops[0].def.pkg.Module != nil {
		title = path.Base(ops[0].def.pkg.Module.Path)
	}
	doc := map[string]any{
		"openapi":    "3.1.0",
		"info":       map[string]any{"title": title, "version": "1"},
		"paths":      paths,
		"components": map[string]any{"schemas": map[string]any{"Errors": errorsSchema}},
	}
	data, _ := json.MarshalIndent(doc, "", "  ")
	return append(data, '\n')
}

// hasBody reports whether a request of the method has a JSON body.
func hasBody(method string) bool {
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// tsOf writes a JSON Schema as a TypeScript type.
func tsOf(schema map[string]any, indent string) string {
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		parts := make([]string, len(enum))
		for i, v := range enum {
			data, _ := json.Marshal(v)
			parts[i] = string(data)
		}
		return strings.Join(parts, " | ")
	}
	switch schema["type"] {
	case "string":
		return "string"
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		items, _ := schema["items"].(map[string]any)
		item := tsOf(items, indent)
		if strings.Contains(item, " | ") {
			item = "(" + item + ")"
		}
		return item + "[]"
	case "object":
		props, ok := schema["properties"].(map[string]any)
		if !ok {
			if extra, ok := schema["additionalProperties"].(map[string]any); ok {
				return "Record<string, " + tsOf(extra, indent) + ">"
			}
			return "Record<string, unknown>"
		}
		if len(props) == 0 {
			return "Record<string, never>"
		}
		required := map[string]bool{}
		if req, ok := schema["required"].([]any); ok {
			for _, name := range req {
				required[name.(string)] = true
			}
		}
		names := make([]string, 0, len(props))
		for name := range props {
			names = append(names, name)
		}
		sort.Strings(names)
		var b strings.Builder
		b.WriteString("{\n")
		for _, name := range names {
			key := name
			if !tsIdent.MatchString(name) {
				key = strconv.Quote(name)
			}
			if !required[name] {
				key += "?"
			}
			sub, _ := props[name].(map[string]any)
			b.WriteString(indent + "  " + key + ": " + tsOf(sub, indent+"  ") + "\n")
		}
		b.WriteString(indent + "}")
		return b.String()
	}
	return "unknown"
}

// camel writes a tool name such as "cart_add" as "cartAdd", or "CartAdd"
// with upper set.
func camel(name string, upper bool) string {
	var b strings.Builder
	for i, part := range strings.Split(name, "_") {
		if part == "" {
			continue
		}
		if i == 0 && !upper {
			b.WriteString(part)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

const tsClientHead = `// Code generated by gx api. DO NOT EDIT.

// GxError is one error of a failed call. A rule failure names the field.
export interface GxError {
  key: string
  field?: string
  message?: string
}

// GxAPIError is a call that the server did not run to its end.
export class GxAPIError extends Error {
  constructor(
    public status: number,
    public errors: GxError[],
  ) {
    super(errors[0]?.message ?? errors[0]?.key ?? 'the call failed with status ' + status)
  }
}

// ClientOptions sets the server of the calls and the headers of each call,
// for example the Authorization header.
export interface ClientOptions {
  baseURL?: string
  headers?: Record<string, string> | (() => Record<string, string> | Promise<Record<string, string>>)
  fetch?: typeof fetch
}

type Query = Record<string, unknown>

async function call<T>(options: ClientOptions, method: string, path: string, query: Query, body: Record<string, unknown> | undefined): Promise<T> {
  const params = new URLSearchParams()
  for (const [name, value] of Object.entries(query)) {
    if (value !== undefined) params.set(name, String(value))
  }
  const search = params.toString()
  const extra = typeof options.headers === 'function' ? await options.headers() : (options.headers ?? {})
  const headers: Record<string, string> = { Accept: 'application/json', ...extra }
  const init: RequestInit = { method, headers }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  const res = await (options.fetch ?? fetch)((options.baseURL ?? '') + path + (search === '' ? '' : '?' + search), init)
  if (res.status === 204) return undefined as T
  let data: unknown
  try {
    data = await res.json()
  } catch {
    data = undefined
  }
  if (!res.ok) throw new GxAPIError(res.status, (data as { errors?: GxError[] } | undefined)?.errors ?? [])
  return data as T
}
`

// tsClient writes the TypeScript client of the actions.
func tsClient(ops []apiOp) []byte {
	var b strings.Builder
	b.WriteString(tsClientHead)
	var methods []string
	for _, op := range ops {
		name := toolName(op.def)
		typeName := camel(name, true)
		b.WriteString("\n")
		if doc := op.def.tool.doc; doc != "" {
			b.WriteString("// " + doc + "\n")
		}
		// An action with no argument has a function with no input.
		props, _ := op.schema["properties"].(map[string]any)
		param := ""
		if len(props) > 0 {
			param = "input: " + typeName + "Input"
			b.WriteString("export type " + typeName + "Input = " + tsOf(op.schema, "") + "\n")
		}
		result := "void"
		if op.result != nil {
			result = typeName + "Result"
			if param != "" {
				b.WriteString("\n")
			}
			b.WriteString("export type " + result + " = " + tsOf(op.result, "") + "\n")
		}
		// The address: each path variable from the input.
		address := "`" + pathVarRe.ReplaceAllString(strings.TrimSpace(strings.SplitN(op.def.pattern, " ", 2)[1]), "${encodeURIComponent(String(input.$1))}") + "`"
		var query, body []string
		for _, f := range op.fields {
			field := "input." + f[0]
			if !tsIdent.MatchString(f[0]) {
				field = "input[" + strconv.Quote(f[0]) + "]"
			}
			entry := strconv.Quote(f[0]) + ": " + field
			switch {
			case f[1] == "path":
			case f[1] == "query" || !hasBody(op.method):
				query = append(query, entry)
			default:
				body = append(body, entry)
			}
		}
		bodyArg := "undefined"
		if hasBody(op.method) {
			bodyArg = "{ " + strings.Join(body, ", ") + " }"
			if len(body) == 0 {
				bodyArg = "{}"
			}
		}
		queryArg := "{}"
		if len(query) > 0 {
			queryArg = "{ " + strings.Join(query, ", ") + " }"
		}
		methods = append(methods, "    "+camel(name, false)+": ("+param+"): Promise<"+result+"> =>\n      call<"+result+">(options, "+
			strconv.Quote(op.method)+", "+address+", "+queryArg+", "+bodyArg+"),\n")
	}
	b.WriteString("\n// createClient returns one function for each action with .API().\nexport function createClient(options: ClientOptions = {}) {\n  return {\n")
	for _, m := range methods {
		b.WriteString(m)
	}
	b.WriteString("  }\n}\n")
	return []byte(b.String())
}
