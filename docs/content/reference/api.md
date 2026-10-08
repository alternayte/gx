---
title: "The gx package"
description: "Each exported function, type, constant and variable of package gx."
section: Reference
order: 1
generated: api
---

Package `github.com/alternayte/gx` is the whole public API of the framework. `just docs-gen` writes this page from the source. The text of each entry is the doc comment of the symbol.

Generated code calls some of these symbols. An app does not call a symbol whose name starts with `Gx`.

## Functions

### func Action

```go
func Action[In any](fn func(*Ctx, In) error) *action[In]
```

Action registers a handler for a route type of any method. The handler answers with Patch, SetSignals, Redirect, Toast or nothing.

### func AllowCredentials

```go
func AllowCredentials(origins ...string) originsOption
```

AllowCredentials lists the origins that can call the routes that follow it in a Group from a different origin with the cookies of the user. It takes exact origins only: a wildcard trusts each later subdomain with the session of each user. Use it for an app of your own on a different subdomain.

### func AllowOrigins

```go
func AllowOrigins(origins ...string) originsOption
```

AllowOrigins lists the origins that can call the routes that follow it in a Group from a different origin. An origin is exact ("https://shop.example.com") , the subdomains of a host ("https://*.partner.io") , or AnyOrigin. A request from a listed origin carries no cookie: the server removes the Cookie header before the middleware runs. The user of such a request comes from a token.

### func AppendJSONBool

```go
func AppendJSONBool(b []byte, v bool) []byte
```

AppendJSONBool appends a JSON boolean.

### func AppendJSONFloat

```go
func AppendJSONFloat(b []byte, f float64) []byte
```

AppendJSONFloat appends a JSON number in the form encoding/json writes. JSON has no NaN and no infinity: such a value is null, and a panic in dev.

### func AppendJSONFloat32

```go
func AppendJSONFloat32(b []byte, f float32) []byte
```

AppendJSONFloat32 appends a float32 as a JSON number in the form encoding/json writes for a float32: the shortest text that gives the same float32. The text of the float64 value has more digits, for example 0.10000000149011612 for 0.1.

### func AppendJSONInt

```go
func AppendJSONInt(b []byte, v int64) []byte
```

AppendJSONInt appends a JSON number. In dev it panics for an integer outside the 53-bit safe range, because the browser rounds it.

### func AppendJSONSignalRef

```go
func AppendJSONSignalRef(b []byte, path string) []byte
```

AppendJSONSignalRef appends a gx.SignalRef prop of an island. The island loader turns the object into a reference that the context of the island resolves.

### func AppendJSONString

```go
func AppendJSONString(b []byte, s string) []byte
```

AppendJSONString appends s as a JSON string. It escapes <, > and & so the text is safe in a script, and writes an invalid UTF-8 byte as U+FFFD.

### func AppendJSONTime

```go
func AppendJSONTime(b []byte, t time.Time) []byte
```

AppendJSONTime appends a time as a JSON string in RFC 3339 form, which the Date constructor of JavaScript reads.

### func AppendJSONUint

```go
func AppendJSONUint(b []byte, v uint64) []byte
```

AppendJSONUint appends a JSON number. In dev it panics for an integer outside the 53-bit safe range, because the browser rounds it.

### func BasePath

```go
func BasePath() string
```

BasePath returns the configured link prefix.

### func BindSignal

```go
func BindSignal(signals map[string]any, scope, name string, dst any) error
```

BindSignal fills dst from the named signal inside scope, for example scope "cart.Cart" and name "qty". A signal that is not in the request leaves dst unchanged.

### func CSP

```go
func CSP(opt CSPOptions) func(http.Handler) http.Handler
```

CSP is middleware that sets a strict Content-Security-Policy with one fresh nonce per response. It works around a whole app, in an app.Group call and around any handler that calls gx.Render.

### func CSRF

```go
func CSRF(next http.Handler) http.Handler
```

CSRF protects non-GET requests with Go's cross-origin protection, plus a token for browser-shaped requests that carry no Fetch Metadata. gx.App applies it to every route. An action or a form on another router applies the cross-origin protection to itself, so no adoption level needs middleware for it. The token needs the cookie that this middleware sets, so mount it around another router to protect browsers without Fetch Metadata.

### func Classes

```go
func Classes(parts ...string) string
```

Classes joins the non-empty parts with single spaces, in source order. Resolve conflicts with gx.Cx.

### func Collection

```go
func Collection[Meta any](dir string) *collection[Meta]
```

Collection declares a content collection rooted at dir, relative to the module root.

### func Cx

```go
func Cx(parts ...string) string
```

Cx merges class strings with the semantics of tailwind-merge for Tailwind v4. A later class removes an earlier class of the same class group or of a group that conflicts with it. A modifier scopes the conflict. The classes that stay keep their order, and a class that Tailwind does not know stays.

### func DefaultMessage

```go
func DefaultMessage(key string) string
```

DefaultMessage returns the English message of a built-in rule key, or the key itself when the key has no default.

### func FieldError

```go
func FieldError[T any](field *T, key string) error
```

FieldError re-renders the form with key as the error of one input field. The field pointer names the field through the generated GxFieldName method.

### func FieldID

```go
func FieldID(form, path string) string
```

FieldID returns the element id of one form field, for example ("signup", "addresses[0].street") gives "signup-addresses-0-street".

### func FieldNames

```go
func FieldNames(errs map[string]string) []string
```

FieldNames returns the sorted field names of a field error map.

### func Forbidden

```go
func Forbidden() error
```

Forbidden tells the page to answer 403.

### func Form

```go
func Form[In any, P any](fn func(*Ctx, In) error, view func(P) Node) *form[In, P]
```

Form registers a form action for a route type. In is a pointer to the route input type:

### func FormIndexes

```go
func FormIndexes(r *http.Request, prefix string) []int
```

FormIndexes returns the sorted row indexes present for an indexed form name, for example 0 and 2 for addresses[0].street and addresses[2].city.

### func FormText

```go
func FormText(r *http.Request, name string) string
```

FormText returns one request form value after parsing.

### func FragmentID

```go
func FragmentID(component, name string, key Key) string
```

FragmentID returns the id of one fragment instance: component-scoped, and keyed when the instance has a key.

### func IsDev

```go
func IsDev() bool
```

IsDev reports whether the dev checks are on.

### func IsIconBody

```go
func IsIconBody(body string) bool
```

IsIconBody reports whether a string looks like the inner markup of an icon. The icon generator refuses anything else, so a generated icon cannot carry markup that breaks out of the svg.

### func IsZero

```go
func IsZero[T comparable](v T) bool
```

IsZero reports whether v is the zero value of its type. The generated encoder of an island calls it for a json omitzero option.

### func JSON

```go
func JSON(v any) string
```

JSON returns the JSON form of a server value inlined into a client expression. It panics when the value has no JSON form: the compiler must not inline it.

### func Layout

```go
func Layout[P any](load func(*Ctx) (P, error), view func(P, Node) Node) layout[P]
```

Layout builds a layout from an optional loader and a view that takes the loaded props and the page node. Pass a nil load when the layout needs no data.

### func Nav

```go
func Nav(mode Navigation) navOption
```

Nav sets the navigation mode of the routes that follow it in a Group.

### func Nonce

```go
func Nonce(r *http.Request) string
```

Nonce returns the CSP nonce of the request, or "" when no policy set one. A page author needs it only for a script inside trusted raw HTML.

### func NotFound

```go
func NotFound() error
```

NotFound tells the page to answer 404.

### func Once

```go
func Once[T any](c *Ctx, fn func() (T, error)) (T, error)
```

Once runs fn once per request and returns its value. It keys on the call site, so share one call site or extract a helper.

### func Page

```go
func Page[In any, P any](load func(*Ctx, In) (P, error), view func(P) Node) *page[In, P]
```

Page builds a typed page from a loader and a view. The route input type In carries the pattern and the generated Bind method.

### func Params

```go
func Params(fn ParamsFunc) func(http.Handler) http.Handler
```

Params returns middleware that installs a path variable reader for routers that do not fill r.PathValue.

### func ParseBool

```go
func ParseBool(s string) (bool, error)
```

ParseBool parses an HTML form boolean. "on" is the value of a checked checkbox without a value attribute.

### func PathValue

```go
func PathValue(r *http.Request, name string) string
```

PathValue reads a path variable through the Params reader, or through r.PathValue when no reader is set.

### func Redirect

```go
func Redirect[In interface{ URL() string }](to In) error
```

Redirect tells the page to answer 303 with a Location.

### func RefPath

```go
func RefPath[T any](r SignalRef[T]) string
```

RefPath returns the adapter reference of a signal ref, with the leading $, so a child can share a parent signal.

### func Render

```go
func Render(w http.ResponseWriter, r *http.Request, n Node) error
```

Render renders n inside any http.Handler. It sets the content type, marks active links for r and writes the merged head.

### func RenderNode

```go
func RenderNode(w io.Writer, n Node) error
```

RenderNode writes n to a writer with no request in scope.

### func RenderRequest

```go
func RenderRequest(w io.Writer, r *http.Request, n Node) error
```

RenderRequest renders n with the request in scope, so typed links mark the active page. The rendered markers feed the runtime script decision of the app.

### func Scope

```go
func Scope(r *http.Request) string
```

Scope returns the signal scope of the invoking component instance, or "".

### func ScopeString

```go
func ScopeString(base string, key Key) string
```

ScopeString returns the signal namespace of a component instance: the component path plus its key.

### func SetAdapter

```go
func SetAdapter(a Adapter)
```

SetAdapter sets the adapter of the process. New does this from Config.Adapter. Actions mounted outside a gx.App use the default.

### func SetBasePath

```go
func SetBasePath(path string)
```

SetBasePath sets the prefix of generated links.

### func SetDev

```go
func SetDev(on bool)
```

SetDev turns the dev checks on or off. gx dev sets it.

### func SetFrontmatterDecoder

```go
func SetFrontmatterDecoder(fn func([]byte, any) error)
```

SetFrontmatterDecoder installs the frontmatter decoder of the process. The gx/content package installs one.

### func SetGallery

```go
func SetGallery(fixtures []Fixture)
```

SetGallery installs the fixture list of the dev gallery. The generated gxdev_gallery package supplies it; the scaffold's main calls this with gxdev.

### func SetIslands

```go
func SetIslands(b IslandBundle)
```

SetIslands installs the island bundle of the app.

### func SetStylesheet

```go
func SetStylesheet(css []byte)
```

SetStylesheet installs the app stylesheet.

### func SetTranslator

```go
func SetTranslator(t Translator)
```

SetTranslator sets the message translator of the process.

### func SetWidgetStylesheets

```go
func SetWidgetStylesheets(sheets map[string][]byte)
```

SetWidgetStylesheets installs the stylesheet of each widget, by the tag of the widget. The main of an app calls it with gxstyles.Widgets(). The stylesheet of a widget holds only the classes that the widget uses, and lives in the shadow root of the element.

### func SignalJSON

```go
func SignalJSON(base string, key Key, values map[string]any) string
```

SignalJSON renders the data-signals JSON of one component instance from the initial values of its signals.

### func SignalName

```go
func SignalName(base string, key Key, name string) string
```

SignalName returns the dotted name of one signal, as data-bind takes it, for example cart.Cart.42.qty.

### func SignalPath

```go
func SignalPath(base string, key Key, name string) string
```

SignalPath returns the adapter signal reference of one signal in one component instance, for example $["cart"]["Cart"]["42"]["qty"].

### func SignalRefPath

```go
func SignalRefPath(base string, key Key, name string) string
```

SignalRefPath returns the bracket path of one signal, without the leading $, for example ["cart"]["Cart"]["42"]["qty"].

### func Signals

```go
func Signals(r *http.Request) (map[string]any, error)
```

Signals decodes the request signals through the app adapter. It returns nil when no adapter is set.

### func SortedKeys

```go
func SortedKeys[K ~string, V any](m map[K]V) []K
```

SortedKeys returns the keys of a map in order, so that the props of an island render the same bytes each time.

### func StaticInputs

```go
func StaticInputs(h Handler) ([]any, bool, error)
```

StaticInputs reports the export inputs of a route, when it lists any.

### func String

```go
func String(n Node) string
```

String returns the HTML of n with no request in scope.

### func StringRequest

```go
func StringRequest(r *http.Request, n Node) string
```

StringRequest returns the HTML of n with the request in scope.

### func Stylesheet

```go
func Stylesheet() []byte
```

Stylesheet returns the installed stylesheet, or nil.

### func TextValue

```go
func TextValue(v any) string
```

TextValue returns the text form of a renderable value.

### func Transition

```go
func Transition[K any](name string) func(K) TransitionName
```

Transition returns a typed transition. Call it with a key to pair one element across two pages:

### func Translate

```go
func Translate(key, fallback string) string
```

Translate returns the message of key. Without a translator it returns fallback, the English default.

### func ValidateURL

```go
func ValidateURL(action, field string) string
```

ValidateURL returns the live validation URL of one field.

### func ViolationKey

```go
func ViolationKey(err error) string
```

ViolationKey returns the message key of a field violation, or "invalid".

### func When

```go
func When(name string, on bool) string
```

When returns name when on is true, and the empty string otherwise.

### func Widget

```go
func Widget[In any, P any](load func(*Ctx, In) (P, error), view func(P) Node) *widget[In, P]
```

Widget builds a widget from a loader and a view. A widget is a component that a page of a different site uses as a custom element. This server renders it.

### func WithNonce

```go
func WithNonce(r *http.Request, nonce string) *http.Request
```

WithNonce returns r with the CSP nonce of the response in scope. Every script element Gx renders for r then carries it. gx.CSP calls it; an app with its own policy middleware calls it with its own nonce.

## Types

### type Adapter

```go
type Adapter interface {
    // Name identifies the adapter, for example "datastar".
    Name() string
    // Signals reports whether the adapter supports client signals and
    // client expressions (REQ-ACT-09).
    Signals() bool
    // Runtime returns the script nodes every page needs (request
    // lifecycle step 6), or nil.
    Runtime() Node
    // Assets returns static files keyed by their name below /_gx/.
    Assets() map[string][]byte
    // Respond writes the commands of an action or a navigation.
    Respond(w http.ResponseWriter, r *http.Request, res *Response) error
    // ReadSignals decodes the request signals into dst, a pointer.
    ReadSignals(r *http.Request, dst any) error
    // Invoke returns the attribute that makes an element invoke the
    // action at url with the HTTP method on its click (REQ-ACT-02). A
    // non-empty scope names the invoking component instance
    // (REQ-ACT-03). It returns the zero Attr for a method the client
    // cannot invoke.
    Invoke(method, url, scope string) Attr
    // On returns the attributes that make an element invoke an action
    // on an event, with the event modifiers (REQ-ACT-08). It returns
    // nil for an invocation that the adapter cannot express.
    On(inv Invocation) []Attr
}
```

Adapter is the public hook between Gx and one hypermedia library. One adapter serves one app (P2) .

#### func AdapterOf

```go
func AdapterOf(r *http.Request) Adapter
```

AdapterOf returns the adapter of the request, or the process default.

### type App

```go
type App struct {
    // contains filtered or unexported fields
}
```

App is an http.Handler that owns a ServeMux.

#### func New

```go
func New(cfg Config) *App
```

New returns an empty app.

#### func (App) Errors

```go
func (a *App) Errors(notFound, forbidden, serverError func(*Ctx) Node) *App
```

Errors sets the error components per status.

#### func (App) Group

```go
func (a *App) Group(prefix string, parts ...any) *App
```

Group mounts routes under a prefix. Middleware applies to the routes that follow it in the same call. gx.Nav selects the navigation mode of the routes that follow it. gx.AllowOrigins and gx.AllowCredentials list the origins that can call the routes that follow them from a different origin.

#### func (App) ServeHTTP

```go
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

ServeHTTP serves the app with cross-origin protection. A page response is buffered so the adapter runtime can join it; action responses stream through (request lifecycle step 6 and 7) .

### type Attr

```go
type Attr struct {
    Key    string
    Value  string
    Kind   AttrKind
    Active string
}
```

Attr is one attribute of an element. Active marks a typed link for the aria-current and data-active rules of REQ-RTE-13: "page" (exact match) or "section" (path prefix) .

#### func Bool

```go
func Bool(key string, present bool) Attr
```

Bool returns a boolean attribute that is omitted when present is false.

#### func ElementModule

```go
func ElementModule(module string) Attr
```

ElementModule returns the attribute that names the entry file of an imported web component. module is the import specifier of gx.lock. The generated code of a web component tag calls it. The render puts the URL of the entry file in the attribute.

#### func Invoke

```go
func Invoke(method, url, scope string) Attr
```

Invoke returns the attribute that invokes the action at url with the HTTP method. A non-empty scope names the invoking component instance. Generated code puts the Value in the attribute of an on: handler. The adapter of the request, or the process default, writes the attribute when the node renders.

#### func On

```go
func On(spec, method, url, scope string) Attr
```

On returns the attribute that invokes an action on an event. Generated code calls it for an on: handler that is one route literal. spec is the text after "on:", for example "click.debounce(300ms)". The adapter of the request, or the process default, writes its own attributes when the node renders.

### type AttrKind

```go
type AttrKind uint8
```

AttrKind selects the escaping rule of an attribute value.

```go
const (
    // AttrText is a plain attribute value.
    AttrText AttrKind = iota
    // AttrURL is a URL attribute value.
    AttrURL
    // AttrBool is a boolean attribute: present when Value is "true".
    AttrBool
    // AttrStyle is a style attribute value.
    AttrStyle
)
```

### type Attrs

```go
type Attrs []Attr
```

Attrs is an ordered attribute list.

#### func FieldControlAttrs

```go
func FieldControlAttrs(f FieldView, describedBy ...string) Attrs
```

FieldControlAttrs returns the control attributes of one field with extra aria-describedby ids, for example a hint element.

#### func JoinAttrs

```go
func JoinAttrs(parts ...Attrs) Attrs
```

JoinAttrs concatenates attribute lists in order.

#### func ToastAttrs

```go
func ToastAttrs(p ToastPatch) Attrs
```

ToastAttrs returns the attributes the root element of a toast carries: the contract between the markup and the behaviour runtime. A component that renders a toast spreads them on its root element.

### type BindError

```go
type BindError struct {
    Err error
}
```

BindError marks a request whose input did not bind; the page answers 400.

#### func (BindError) Error

```go
func (e *BindError) Error() string
```

#### func (BindError) Unwrap

```go
func (e *BindError) Unwrap() error
```

### type Binder

```go
type Binder interface {
    Pattern() string
    Bind(*http.Request) error
}
```

Binder is the generated route interface: a pattern and request binding. The gx generator writes Pattern and Bind.

### type Builder

```go
type Builder struct {
    // contains filtered or unexported fields
}
```

Builder collects nodes while a generated component runs.

#### func (Builder) Add

```go
func (b *Builder) Add(n ...Node)
```

Add appends nodes to the builder.

#### func (Builder) Node

```go
func (b *Builder) Node() Node
```

Node returns the built node.

### type CSPOptions

```go
type CSPOptions struct {
    // UnsafeEval adds 'unsafe-eval' to script-src. The Datastar adapter
    // needs it: Datastar evaluates client expressions at runtime.
    UnsafeEval bool
    // Directives holds more directives, for example
    // "img-src 'self' data:; frame-ancestors 'none'".
    Directives string
}
```

CSPOptions configure gx.CSP.

### type Code

```go
type Code struct {
    // File is the module-relative path of the source file.
    File string
    // Lang is the chroma lexer name; empty guesses from File.
    Lang string
    // Source is the selected source text with a trailing newline.
    Source string
}
```

Code is one code block of the docs kit. The compiler fills it from gx.CodeFile at build time and fails the build when the file or a selected line is missing.

#### func CodeFile

```go
func CodeFile(path, lines string) Code
```

CodeFile names a repository file for a code block. The compiler replaces the call with the resolved gx.Code value. The values here keep a generated file type-checking and make the intent explicit.

### type Config

```go
type Config struct {
    BasePath string
    Adapter  Adapter
    // Toast renders one toast (REQ-REG-11). An app sets it to the Render
    // function of its installed toast item, so the toast markup and its
    // classes stay in app-owned source. Without it a toast is the plain
    // ToastNode.
    Toast func(ToastPatch) Node
    // Public holds the app's own static files, for example an embedded
    // public directory. A GET for a path that names a file in it answers
    // that file before any route, so the binary needs no file beside it
    // (NFR-08).
    Public fs.FS
}
```

Config holds app options. BasePath prefixes every generated link, and Adapter selects the hypermedia library. Package gx exports only the standard library plus an optional adapter runtime dependency.

### type ContentError

```go
type ContentError struct {
    File string
    Msg  string
}
```

ContentError is a frontmatter or rule error of one content file.

#### func (ContentError) Error

```go
func (e *ContentError) Error() string
```

### type ContentPage

```go
type ContentPage struct {
    Slug string `path:"slug"`
    // contains filtered or unexported fields
}
```

ContentPage is the export input of one content page.

#### func (ContentPage) URL

```go
func (p ContentPage) URL() string
```

URL implements the typed link value of a content page. The index entry is the root of the collection.

### type ContentRoute

```go
type ContentRoute[Meta any] struct {
    // contains filtered or unexported fields
}
```

ContentRoute serves one content collection and carries the configuration of the llms.txt export.

#### func ContentEntries

```go
func ContentEntries[Meta any](c *collection[Meta], view func(Entry[Meta]) Node) *ContentRoute[Meta]
```

ContentEntries makes one route per entry and passes the whole Entry to view. The docs shell needs the slug to build links, the table of contents and the previous and next pages.

#### func ContentPages

```go
func ContentPages[Meta any](c *collection[Meta], view func(Meta, []byte) Node) *ContentRoute[Meta]
```

ContentPages makes the content route of the collection: one URL per entry under "/{slug...}". view builds the page from the typed frontmatter and the raw Markdown body; the docs kit renders the body.

#### func (ContentRoute[Meta]) LLMS

```go
func (h *ContentRoute[Meta]) LLMS(opt LLMSOptions[Meta]) *ContentRoute[Meta]
```

LLMS configures the llms.txt files of this collection.

#### func (ContentRoute[Meta]) Pattern

```go
func (h *ContentRoute[Meta]) Pattern() string
```

Pattern implements Handler. The trailing wildcard serves a nested entry slug ("guides/routing") as one path.

#### func (ContentRoute[Meta]) ServeHTTP

```go
func (h *ContentRoute[Meta]) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

ServeHTTP renders the entry named by the path value. The empty slug is the collection index.

### type Ctx

```go
type Ctx struct {
    W http.ResponseWriter
    R *http.Request
    // contains filtered or unexported fields
}
```

Ctx is the per-request context.

#### func (Ctx) Patch

```go
func (c *Ctx) Patch(nodes ...Node) error
```

Patch sends fragment patches by id. The default mode is morph.

#### func (Ctx) Redirect

```go
func (c *Ctx) Redirect(to interface{ URL() string }) error
```

Redirect answers with a client navigation to a route value.

#### func (Ctx) SetSignals

```go
func (c *Ctx) SetSignals(v any) error
```

SetSignals updates the signals of the invoking component instance.

#### func (Ctx) Toast

```go
func (c *Ctx) Toast(text string, opts ...ToastOption) error
```

Toast adds a toast to the toaster region. A kind is an option: c.Toast("Saved", gx.ToastSuccess).

### type ElementPatch

```go
type ElementPatch struct {
    Mode       PatchMode
    Target     string
    Node       Node
    Transition bool
}
```

ElementPatch changes one element, named by Target, a CSS selector.

### type Entry

```go
type Entry[Meta any] struct {
    // Slug is the path below the collection directory without ".md".
    Slug string
    // File is the absolute path of the file.
    File string
    // Meta is the decoded frontmatter.
    Meta Meta
    // Body is the Markdown without the frontmatter.
    Body []byte
}
```

Entry is one loaded content page.

### type Enum

```go
type Enum[T comparable] map[T]string
```

Enum[T] is a class map for the constants of T. The analyzer requires an entry for every constant of T.

### type FieldView

```go
type FieldView interface {
    Attrs() Attrs
    FieldName() string
    FieldID() string
    FieldValue() string
    FieldError() string
    FieldErrorKey() string
    FieldValidateURL() string
    // FieldInputType returns the rule-derived input type ("email", "url"),
    // or fallback (REQ-FRM-04).
    FieldInputType(fallback string) string
    // FieldHint returns the helper text, or "".
    FieldHint() string
}
```

FieldView is the non-generic view of a field, for field components that serve every field type.

### type FieldViolation

```go
type FieldViolation struct {
    Field   string
    Key     string
    Message string
}
```

FieldViolation is one failed field rule.

#### func RunAllRulesContext

```go
func RunAllRulesContext(ctx context.Context, v any) []*FieldViolation
```

RunAllRulesContext runs every field rule and returns one failure per failing field, in rule order. The form handler shows the whole error list.

#### func RunRules

```go
func RunRules(v any) *FieldViolation
```

RunRules runs the Rules of an input value and returns the first failure.

#### func RunRulesContext

```go
func RunRulesContext(ctx context.Context, v any) *FieldViolation
```

RunRulesContext is RunRules with a request context for CheckCtx.

#### func (FieldViolation) Error

```go
func (e *FieldViolation) Error() string
```

### type File

```go
type File struct {
    // Name is the client file name.
    Name string
    // Size is the number of bytes stored.
    Size int64
    // Type is the content type, for example "image/png".
    Type string
    // Temp is the path of the temp file.
    Temp string
}
```

File is one uploaded file. The upload streams to a temp file during binding; the framework removes it after the handler returns.

#### func ReadUploads

```go
func ReadUploads(r *http.Request, name string, maxSize int64, patterns []string) ([]File, error)
```

ReadUploads streams every part of one form name to a temp file and enforces the size and type limits before the handler runs.

#### func (File) Open

```go
func (f File) Open() (io.ReadCloser, error)
```

Open opens the uploaded file for reading.

#### func (File) Remove

```go
func (f File) Remove() error
```

Remove deletes the temp file.

### type Fixture

```go
type Fixture struct {
    Component string
    // Package is the import path of the component, for the dev gallery.
    Package string
    Name    string
    Node    func() Node
    Missing bool
}
```

Fixture is one gallery entry. A component without a fixtures file has Missing set and no Node.

#### func Gallery

```go
func Gallery() []Fixture
```

Gallery returns a copy of the installed fixture list.

### type Fixtures

```go
type Fixtures[Props any] map[string]Props
```

Fixtures names example prop sets for the dev gallery. A file `<Name>`.fixtures.go declares one:

### type FormField

```go
type FormField[T any] struct {
    // Name is the form field name, for example "email".
    Name string
    // ID is the control id, for example "signup-email".
    ID string
    // Value is the bound value.
    Value T
    // Error is the translated error message, or "".
    Error string
    // ErrorKey is the stable message key of Error, for example "required".
    ErrorKey string
    // Constraints holds the native constraints of the rules
    // (REQ-FRM-04).
    Constraints Attrs
    // ValidateURL is the URL of the live validation action (REQ-FRM-06).
    ValidateURL string
    // Hint is the helper text of the control, or "".
    Hint string
}
```

Field is one generated form control. The compiler writes the static parts; the form handler writes Error and ErrorKey.

#### func (FormField[T]) Attrs

```go
func (f FormField[T]) Attrs() Attrs
```

Attrs returns the control attributes: name, id, value, native constraints and the aria state.

#### func (FormField[T]) FieldError

```go
func (f FormField[T]) FieldError() string
```

FieldError implements FieldView.

#### func (FormField[T]) FieldErrorKey

```go
func (f FormField[T]) FieldErrorKey() string
```

FieldErrorKey implements FieldView.

#### func (FormField[T]) FieldHint

```go
func (f FormField[T]) FieldHint() string
```

FieldHint implements FieldView.

#### func (FormField[T]) FieldID

```go
func (f FormField[T]) FieldID() string
```

FieldID implements FieldView.

#### func (FormField[T]) FieldInputType

```go
func (f FormField[T]) FieldInputType(fallback string) string
```

FieldInputType implements FieldView.

#### func (FormField[T]) FieldName

```go
func (f FormField[T]) FieldName() string
```

FieldName implements FieldView.

#### func (FormField[T]) FieldValidateURL

```go
func (f FormField[T]) FieldValidateURL() string
```

FieldValidateURL implements FieldView.

#### func (FormField[T]) FieldValue

```go
func (f FormField[T]) FieldValue() string
```

FieldValue implements FieldView.

### type FormInput

```go
type FormInput interface {
    Pattern() string
    // GxNewForm returns a fresh input value.
    GxNewForm() FormInput
    GxBindForm(*http.Request) (map[string]string, error)
    Rules() Rules
    GxFormValue(map[string]string) FormValue
    GxFieldName(any) string
    // GxRunForm calls the user handler with the concrete input type.
    GxRunForm(*Ctx, any) error
}
```

FormInput is the generated interface of a form input type.

### type FormMeta

```go
type FormMeta struct {
    // Name is the input type in lower-first form, for example "signup".
    Name string
    // ID is the id of the form element, for example "signup-form".
    ID string
    // Action is the form action URL, from the route pattern.
    Action string
    // Method is the HTTP method, for example "POST".
    Method string
    // Enctype is the form encoding when the form holds files
    // (REQ-FRM-09).
    Enctype string
}
```

FormMeta is the generated form element state of an input type. The compiler embeds it in every `<Type>`Form value.

#### func (FormMeta) Attrs

```go
func (m FormMeta) Attrs() Attrs
```

Attrs returns the attributes of the form element. The client runtime submits every form marked with data-gx-form through the adapter.

#### func (FormMeta) GxFormAction

```go
func (m FormMeta) GxFormAction() string
```

GxFormAction implements FormValue.

#### func (FormMeta) GxFormID

```go
func (m FormMeta) GxFormID() string
```

GxFormID implements FormValue.

#### func (FormMeta) GxFormMethod

```go
func (m FormMeta) GxFormMethod() string
```

GxFormMethod implements FormValue.

#### func (FormMeta) GxFormName

```go
func (m FormMeta) GxFormName() string
```

GxFormName implements FormValue.

### type FormProps

```go
type FormProps interface {
    GxSetForm(FormValue)
}
```

FormProps is implemented by a generated view props struct that holds a form value. The generated GxSetForm fills that field.

### type FormValue

```go
type FormValue interface {
    GxFormName() string
    GxFormID() string
    GxFormAction() string
    GxFormMethod() string
}
```

FormValue is implemented by every generated `<Type>`Form.

### type Handler

```go
type Handler interface {
    http.Handler
    Pattern() string
}
```

Handler is one typed route: a pattern plus an http.Handler.

#### func Collect

```go
func Collect(hs ...Handler) []Handler
```

Collect returns its arguments as one route list.

### type HeadProps

```go
type HeadProps struct {
    Title string `json:"title,omitempty"`
    Meta  []Meta `json:"meta,omitempty"`
    Links []Link `json:"links,omitempty"`
    // Lang, HtmlClass and BodyClass set attributes of the document shell.
    // The deepest value wins; Lang defaults to "en".
    Lang      string `json:"lang,omitempty"`
    HtmlClass string `json:"htmlClass,omitempty"`
    BodyClass string `json:"bodyClass,omitempty"`
}
```

HeadProps is the props of the gx.Head component.

#### func HeadOf

```go
func HeadOf(n Node) HeadProps
```

HeadOf returns the merged head of a node tree.

### type IconProps

```go
type IconProps struct {
    // Label sets role="img" and aria-label. Without it the icon is
    // decorative: aria-hidden="true".
    Label string
    // Class is the class attribute of the svg.
    Class string
    // Attrs adds extra attributes.
    Attrs Attrs
}
```

IconProps are the props of a generated icon component.

### type Invocation

```go
type Invocation struct {
    // Method is the HTTP method and URL is the address of the action.
    Method string
    URL    string
    // Scope names the invoking component instance, or is empty
    // (REQ-ACT-03).
    Scope string
    // Event is the name of a DOM event, or "load", "visible" or
    // "interval".
    Event string
    // Every is the period of an interval event, for example "5s".
    Every string
    // Mods are the event modifiers in source order.
    Mods []Modifier
}
```

Invocation is one client call of an action on an event.

### type IslandBundle

```go
type IslandBundle struct {
    // Entries maps the name of an island to its file.
    Entries map[string]string
    // Files maps a file name to its content. Each name holds the hash of
    // the content.
    Files map[string]string
}
```

IslandBundle is the built JavaScript of the islands of an app. gx build writes it into the generated package gxislands, and the app's main installs it with gx.SetIslands(gxislands.Bundle()).

### type IslandOption

```go
type IslandOption struct {
    // contains filtered or unexported fields
}
```

IslandOption sets how the browser loads an island.

#### func IslandKey

```go
func IslandKey(key Key) IslandOption
```

IslandKey gives one island of a list its identity. The element gets an id from the key. A morph then pairs the island with its own row, and the state of the island follows the row.

#### func IslandLoad

```go
func IslandLoad(strategy string) IslandOption
```

IslandLoad names the time the browser loads the island file: "eager" at once, "idle" when the browser is idle, "visible" when the element comes near the viewport. With no option the island loads when it is visible.

#### func IslandMedia

```go
func IslandMedia(query string) IslandOption
```

IslandMedia loads the island when the media query matches, for example "(min-width: 768px)".

### type Key

```go
type Key string
```

Key identifies one component instance.

#### func ChildKey

```go
func ChildKey(parent Key, index int) Key
```

ChildKey returns the key of an unkeyed call site under parent. The same call site gives the same key on every render.

#### func InstanceKey

```go
func InstanceKey(v any) Key
```

InstanceKey returns the key of a component call site from its key expression.

#### func ScopeKey

```go
func ScopeKey(scope, base string) Key
```

ScopeKey returns the instance key inside a scope for a component base, for example "42" for base "cart.Cart" and scope "cart.Cart.42". It returns "" when the scope is not that component.

### type LLMSEntry

```go
type LLMSEntry struct {
    Path        string `json:"path"`
    Title       string `json:"title"`
    Description string `json:"description"`
    Body        string `json:"body"`
    Skip        bool   `json:"skip"`
}
```

LLMSEntry is one page of the llms.txt export.

### type LLMSManifest

```go
type LLMSManifest struct {
    Site    string      `json:"site"`
    Summary string      `json:"summary"`
    Entries []LLMSEntry `json:"entries"`
}
```

LLMSManifest is the llms.txt data of one content route. The dev-only export manifest carries it to the exporter.

### type LLMSOptions

```go
type LLMSOptions[Meta any] struct {
    // Site is the H1 of llms.txt.
    Site string
    // Summary is the blockquote line of llms.txt.
    Summary string
    // Title returns the entry title.
    Title func(Meta) string
    // Description returns the entry description.
    Description func(Meta) string
    // Skip keeps the entry out of the llms files when it returns true.
    Skip func(Meta) bool
}
```

LLMSOptions configure the llms.txt export of one content route.

### type Link

```go
type Link struct {
    Rel  string `json:"rel,omitempty"`
    Href string `json:"href,omitempty"`
}
```

Link is one link tag in the head.

### type Meta

```go
type Meta struct {
    Name     string `json:"name,omitempty"`
    Property string `json:"property,omitempty"`
    Content  string `json:"content,omitempty"`
}
```

Meta is one meta tag in the head.

### type Modifier

```go
type Modifier struct {
    Name  string
    Value string
}
```

Modifier is one event modifier of an Invocation. Value is the text in its parentheses, for example "300ms" for debounce(300ms).

#### func ParseOn

```go
func ParseOn(spec string) (event, every string, mods []Modifier)
```

ParseOn splits the text after "on:" into an event, the period of an interval event and the modifiers. A dot inside parentheses does not start a modifier.

### type Navigation

```go
type Navigation uint8
```

Navigation selects how a link between two pages with a shared layout loads.

```go
const (
    // FullNavigation loads every link as a full page.
    FullNavigation Navigation = iota
    // MorphNavigation fetches only the slot of the deepest shared layout.
    MorphNavigation
)
```

### type Node

```go
type Node interface {
    // contains filtered or unexported methods
}
```

Node is one node of a render tree. Only this package implements Node.

```go
var Append Node = patchModeNode{ModeAppend}
```

Append sends the following patches with append mode.

```go
var Prepend Node = patchModeNode{ModePrepend}
```

Prepend sends the following patches with prepend mode.

```go
var Remove Node = patchModeNode{ModeRemove}
```

Remove sends the following patches with remove mode.

```go
var Replace Node = patchModeNode{ModeReplace}
```

Replace sends the following patches with replace mode.

```go
var ViewTransition Node = transitionNode{}
```

ViewTransition wraps the following patches in startViewTransition.

#### func DevRender

```go
func DevRender(pkgPath, name string, args ...any) (Node, bool)
```

DevRender is the dev hook of a generated function. A production build never calls it.

#### func EachRow

```go
func EachRow[T any](rows []T, fn func(int, T) Node) Node
```

EachRow renders one node per row of a slice field.

#### func El

```go
func El(name string, attrs Attrs, children ...Node) Node
```

El returns an element node. An empty attribute value means a boolean attribute; AttrBool with the value "false" is omitted.

#### func FieldErrorNode

```go
func FieldErrorNode(fieldID, message string) Node
```

FieldErrorNode renders the error element of one field. Field components render the same element so a live validation patch morphs it.

#### func Frag

```go
func Frag(children ...Node) Node
```

Frag returns a node that renders its children in order.

#### func Head

```go
func Head(p HeadProps) Node
```

Head marks the head of a page or layout. The deepest title wins and the other tags merge.

#### func Icon

```go
func Icon(body string, p IconProps) Node
```

Icon renders one icon as an inline svg with currentColor. body is the inner markup of a pinned icon pack, not user input.

#### func Island

```go
func Island(name, props string, opts ...IslandOption) Node
```

Island returns the element of a TypeScript island. name is the import path of the package and the component name. props is the JSON of the props. The generated component function of an island calls it.

#### func Raw

```go
func Raw(s SafeHTML) Node
```

Raw returns a node that writes s without escaping. Use it only for gx.SafeHTML values.

#### func RenderToast

```go
func RenderToast(r *http.Request, p ToastPatch) Node
```

RenderToast renders one toast for an adapter: with Config.Toast of the app that serves the request, or as ToastNode when the app sets none.

#### func Text

```go
func Text(s string) Node
```

Text returns a node that escapes s as HTML text.

#### func ToastNode

```go
func ToastNode(p ToastPatch) Node
```

ToastNode is the plain markup of one toast. An app that sets Config.Toast renders its own component instead.

#### func Toaster

```go
func Toaster() Node
```

Toaster renders the plain region that Toast patches into.

#### func Value

```go
func Value(v any) Node
```

Value returns a text node for a renderable value: bool, integer, float, string, fmt.Stringer or error.

### type ParamsFunc

```go
type ParamsFunc func(*http.Request, string) string
```

ParamsFunc reads a path variable from a request.

### type Patch

```go
type Patch interface {
    // contains filtered or unexported methods
}
```

Patch is one change an adapter sends to the client.

### type PatchMode

```go
type PatchMode uint8
```

PatchMode selects how an element patch reaches the DOM.

```go
const (
    // ModeMorph morphs the node into the existing element. It is the
    // default.
    ModeMorph PatchMode = iota
    // ModeInner replaces the children of the existing element.
    ModeInner
    // ModeAppend puts the node inside the existing element, at the end.
    ModeAppend
    // ModePrepend puts the node inside the existing element, at the start.
    ModePrepend
    // ModeReplace replaces the existing element with the node.
    ModeReplace
    // ModeRemove removes the existing element.
    ModeRemove
)
```

### type RedirectPatch

```go
type RedirectPatch struct{ URL string }
```

RedirectPatch navigates the client to URL.

### type Response

```go
type Response struct {
    Patches []Patch
    // Status is the HTTP status to answer with, or 0 for the default.
    Status int
    // Err is the handler error, shown by the dev overlay (REQ-DEV-06).
    Err error
    // Navigate marks a partial navigation response (REQ-RTE-12).
    Navigate bool
    // Head is the merged head of a partial navigation (REQ-RTE-12).
    Head *HeadProps
}
```

Response is the ordered answer of an action, a form or a navigation.

### type Route

```go
type Route struct{}
```

Route is embedded in a route input struct. The tag carries the method and the pattern: "GET /products/{id}".

### type Rule

```go
type Rule struct {
    // contains filtered or unexported fields
}
```

Rule checks one input value. Every built-in rule leaves the zero value alone; Required rejects it.

#### func Accept

```go
func Accept(types ...string) Rule
```

Accept limits the content types of an uploaded file.

#### func Check

```go
func Check(fn func(v any) error) Rule
```

Check runs a server-only function.

#### func CheckCtx

```go
func CheckCtx(fn func(ctx context.Context, v any) error) Rule
```

CheckCtx runs a server-only function with the request context.

#### func Each

```go
func Each(rule Rule) Rule
```

Each applies a rule to every element of a slice or array.

#### func Field

```go
func Field[T any](v *T, rules ...Rule) Rule
```

Field binds rules to one field. All rules must pass.

#### func Max

```go
func Max(n int64) Rule
```

Max rejects a numeric value above n.

#### func MaxLen

```go
func MaxLen(n int) Rule
```

MaxLen rejects a string longer than n code points.

#### func MaxSize

```go
func MaxSize(n int64) Rule
```

MaxSize limits an uploaded file in bytes.

#### func Min

```go
func Min(n int64) Rule
```

Min rejects a numeric value below n.

#### func MinLen

```go
func MinLen(n int) Rule
```

MinLen rejects a string shorter than n code points.

#### func OneOf

```go
func OneOf(values ...string) Rule
```

OneOf rejects a string outside the allowed values.

#### func Pattern

```go
func Pattern(re *regexp.Regexp) Rule
```

Pattern rejects a string that does not match a constant regexp.

#### func True

```go
func True(key string) Rule
```

True rejects a false bool.

### type Rules

```go
type Rules []Rule
```

Rules is the rule set of an input type.

### type SafeHTML

```go
type SafeHTML string
```

SafeHTML is HTML that needs no escaping.

### type Secret

```go
type Secret string
```

Secret is a value that must not leave the server. It renders and marshals as "[redacted]".

#### func (Secret) MarshalJSON

```go
func (s Secret) MarshalJSON() ([]byte, error)
```

MarshalJSON never writes the secret.

#### func (Secret) Reveal

```go
func (s Secret) Reveal() string
```

Reveal returns the secret value. Use it on the server only.

#### func (Secret) String

```go
func (s Secret) String() string
```

String returns the redacted form.

### type SignalPatch

```go
type SignalPatch struct {
    Scope   string
    Signals any
}
```

SignalPatch sets the signals of one component scope.

### type SignalRef

```go
type SignalRef[T any] string
```

SignalRef is a prop that carries a parent signal reference into a child component.

#### func Ref

```go
func Ref[T any](path string) SignalRef[T]
```

Ref builds a signal reference value from a bracket path.

### type Slot

```go
type Slot[T any] func(T) Node
```

Slot is a typed slot: a function that renders one value.

### type Style

```go
type Style string
```

Style is a dynamic style attribute value.

#### func StyleJoin

```go
func StyleJoin(parts ...Style) Style
```

StyleJoin joins style parts with a semicolon.

#### func TransitionStyle

```go
func TransitionStyle(t TransitionName) Style
```

TransitionStyle renders the sanitized view-transition-name and view-transition-class of a transition.

### type ToastControl

```go
type ToastControl struct {
    Label string
    URL   URL
    // Method is the HTTP method of the action. It is empty for a link.
    Method string
}
```

ToastControl is the one control of a toast: a link to URL, or a button that invokes the action at URL.

### type ToastKind

```go
type ToastKind uint8
```

ToastKind is the kind of one toast. A kind is also a ToastOption, so an action passes it directly: c.Toast("Saved", gx.ToastSuccess).

```go
const (
    // ToastDefault is a neutral message with no icon.
    ToastDefault ToastKind = iota
    // ToastSuccess reports a finished operation.
    ToastSuccess
    // ToastInfo gives neutral information.
    ToastInfo
    // ToastWarning reports a result the user must check.
    ToastWarning
    // ToastError reports a failure. It renders as role="alert".
    ToastError
    // ToastLoading reports running work. It stays until a later toast with
    // the same ID replaces it, or the user closes it.
    ToastLoading
)
```

#### func (ToastKind) String

```go
func (k ToastKind) String() string
```

String returns the kind as the data-kind value of the toast markup.

### type ToastOption

```go
type ToastOption interface {
    // contains filtered or unexported methods
}
```

ToastOption sets one field of a toast.

```go
var ToastSticky ToastOption = toastOptionFunc(func(p *ToastPatch) { p.Sticky = true })
```

ToastSticky keeps the toast until the user closes it.

#### func ToastAction

```go
func ToastAction[In interface {
    Pattern() string
    URL() string
}](label string, in In) ToastOption
```

ToastAction adds one button that invokes the action of a route value, for example Undo. The toast closes when the user presses the button. A toast holds one control: of ToastLink and ToastAction, the later option wins.

#### func ToastDescription

```go
func ToastDescription(text string) ToastOption
```

ToastDescription adds a second line below the text.

#### func ToastDuration

```go
func ToastDuration(d time.Duration) ToastOption
```

ToastDuration sets how long the toast stays. The default is 4 seconds.

#### func ToastID

```go
func ToastID(id string) ToastOption
```

ToastID names the toast. A later toast with the same ID replaces the earlier one in place, so a handler can show loading and then success.

#### func ToastLink

```go
func ToastLink(label string, to interface{ URL() string }) ToastOption
```

ToastLink adds one link that navigates to a route value. A toast holds one control: of ToastLink and ToastAction, the later option wins.

### type ToastPatch

```go
type ToastPatch struct {
    Text        string
    Kind        ToastKind
    Description string
    // ID makes a later toast with the same ID replace this one in place.
    ID string
    // Duration is the time before the toast leaves, or 0 for the default.
    Duration time.Duration
    // Sticky keeps the toast until the user closes it.
    Sticky bool
    // Action is the control: a link or an action button. The zero value
    // renders none.
    Action ToastControl
}
```

ToastPatch adds one toast to the toaster region.

#### func (ToastPatch) Timeout

```go
func (p ToastPatch) Timeout() time.Duration
```

Timeout returns the time before the toast leaves on its own. It is 0 for a sticky toast and for a loading toast: they never leave on their own.

### type TransitionName

```go
type TransitionName struct {
    Name string
    Key  string
}
```

TransitionName is the rendered value of a typed transition.

### type Translator

```go
type Translator func(key, fallback string) string
```

Translator turns a message key into a message. fallback is the English message of the key.

### type URL

```go
type URL string
```

URL is a prebuilt URL for an href or src.

#### func (URL) URL

```go
func (u URL) URL() string
```

URL implements the redirect target interface.

### type Unchecked

```go
type Unchecked struct{}
```

Unchecked marks an action input whose signal fields need no rules. Embed it in the route struct.

## Constants and variables

### AnyOrigin

```go
const AnyOrigin = "*"
```

AnyOrigin lists each origin in AllowOrigins. Use it for a public widget. AllowCredentials does not take it.

### DefaultThemeCSS

```go
const DefaultThemeCSS = `@import "tailwindcss";

@source "../.gx/classes.txt";
@source not "../js/vendor";
@source not "../gxislands";
@source not "../gxstyles";

@custom-variant dark (&:where(.dark, .dark *));

@layer base {
  * {
    @apply border-border outline-ring/50;
  }
}

/* Cross-document view transitions for full loads (REQ-STY-09). */
@view-transition {
  navigation: auto;
}

:root {
  --radius: 0.625rem;
  --background: oklch(1 0 0);
  --foreground: oklch(0.145 0 0);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.145 0 0);
  --popover: oklch(1 0 0);
  --popover-foreground: oklch(0.145 0 0);
  --primary: oklch(0.205 0 0);
  --primary-foreground: oklch(0.985 0 0);
  --secondary: oklch(0.97 0 0);
  --secondary-foreground: oklch(0.205 0 0);
  --muted: oklch(0.97 0 0);
  --muted-foreground: oklch(0.556 0 0);
  --accent: oklch(0.97 0 0);
  --accent-foreground: oklch(0.205 0 0);
  --destructive: oklch(0.577 0.245 27.325);
  --destructive-foreground: oklch(0.985 0 0);
  --border: oklch(0.922 0 0);
  --input: oklch(0.922 0 0);
  --ring: oklch(0.708 0 0);
  --chart-1: oklch(0.646 0.222 41.116);
  --chart-2: oklch(0.6 0.118 184.704);
  --chart-3: oklch(0.398 0.07 227.392);
  --chart-4: oklch(0.828 0.189 84.429);
  --chart-5: oklch(0.769 0.188 70.08);
  --sidebar: oklch(0.985 0 0);
  --sidebar-foreground: oklch(0.145 0 0);
  --sidebar-primary: oklch(0.205 0 0);
  --sidebar-primary-foreground: oklch(0.985 0 0);
  --sidebar-accent: oklch(0.97 0 0);
  --sidebar-accent-foreground: oklch(0.205 0 0);
  --sidebar-border: oklch(0.922 0 0);
  --sidebar-ring: oklch(0.708 0 0);
}

.dark {
  --background: oklch(0.145 0 0);
  --foreground: oklch(0.985 0 0);
  --card: oklch(0.205 0 0);
  --card-foreground: oklch(0.985 0 0);
  --popover: oklch(0.269 0 0);
  --popover-foreground: oklch(0.985 0 0);
  --primary: oklch(0.922 0 0);
  --primary-foreground: oklch(0.205 0 0);
  --secondary: oklch(0.269 0 0);
  --secondary-foreground: oklch(0.985 0 0);
  --muted: oklch(0.269 0 0);
  --muted-foreground: oklch(0.708 0 0);
  --accent: oklch(0.371 0 0);
  --accent-foreground: oklch(0.985 0 0);
  --destructive: oklch(0.704 0.191 22.216);
  --destructive-foreground: oklch(0.985 0 0);
  --border: oklch(1 0 0 / 10%);
  --input: oklch(1 0 0 / 15%);
  --ring: oklch(0.556 0 0);
  --sidebar: oklch(0.205 0 0);
  --sidebar-foreground: oklch(0.985 0 0);
  --sidebar-primary: oklch(0.488 0.243 264.376);
  --sidebar-primary-foreground: oklch(0.985 0 0);
  --sidebar-accent: oklch(0.269 0 0);
  --sidebar-accent-foreground: oklch(0.985 0 0);
  --sidebar-border: oklch(1 0 0 / 10%);
  --sidebar-ring: oklch(0.556 0 0);
}

@media (prefers-color-scheme: dark) {
  :root:not(.light) {
    --background: oklch(0.145 0 0);
    --foreground: oklch(0.985 0 0);
    --card: oklch(0.205 0 0);
    --card-foreground: oklch(0.985 0 0);
    --popover: oklch(0.269 0 0);
    --popover-foreground: oklch(0.985 0 0);
    --primary: oklch(0.922 0 0);
    --primary-foreground: oklch(0.205 0 0);
    --secondary: oklch(0.269 0 0);
    --secondary-foreground: oklch(0.985 0 0);
    --muted: oklch(0.269 0 0);
    --muted-foreground: oklch(0.708 0 0);
    --accent: oklch(0.371 0 0);
    --accent-foreground: oklch(0.985 0 0);
    --destructive: oklch(0.704 0.191 22.216);
    --destructive-foreground: oklch(0.985 0 0);
    --border: oklch(1 0 0 / 10%);
    --input: oklch(1 0 0 / 15%);
    --ring: oklch(0.556 0 0);
    --sidebar: oklch(0.205 0 0);
    --sidebar-foreground: oklch(0.985 0 0);
    --sidebar-primary: oklch(0.488 0.243 264.376);
    --sidebar-primary-foreground: oklch(0.985 0 0);
    --sidebar-accent: oklch(0.269 0 0);
    --sidebar-accent-foreground: oklch(0.985 0 0);
    --sidebar-border: oklch(1 0 0 / 10%);
    --sidebar-ring: oklch(0.556 0 0);
  }
}

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--chart-2);
  --color-chart-3: var(--chart-3);
  --color-chart-4: var(--chart-4);
  --color-chart-5: var(--chart-5);
  --color-sidebar: var(--sidebar);
  --color-sidebar-foreground: var(--sidebar-foreground);
  --color-sidebar-primary: var(--sidebar-primary);
  --color-sidebar-primary-foreground: var(--sidebar-primary-foreground);
  --color-sidebar-accent: var(--sidebar-accent);
  --color-sidebar-accent-foreground: var(--sidebar-accent-foreground);
  --color-sidebar-border: var(--sidebar-border);
  --color-sidebar-ring: var(--sidebar-ring);
  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);
}
`
```

DefaultThemeCSS is the Tailwind v4 theme file that `gx init` writes to app/theme.css. Token names match shadcn. Dark mode follows a .dark class, and a system preference when no class is set.

### Dev

```go
const Dev = false
```

Dev is false in a production build. Generated code asks it before it calls DevRender, so the compiler removes that call and the binary holds no part of the dev interpreter.

### Email

```go
var Email = Rule{
    // contains filtered or unexported fields
}
```

Email rejects a non-empty string that is not an email address.

### IsURL

```go
var IsURL = Rule{
    // contains filtered or unexported fields
}
```

IsURL rejects a non-empty string that is not an http or https URL. It is named IsURL because gx.URL is the typed link value.

### MaxFormRows

```go
const MaxFormRows = 1000
```

MaxFormRows is the number of rows one repeated form field can hold. A row with a larger index is ignored.

### Required

```go
var Required = Rule{
    // contains filtered or unexported fields
}
```

Required rejects the zero value of a string, number or bool.
