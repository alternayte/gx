package compiler

import (
	"fmt"
	"strconv"
)

// Diagnostic codes. Every code is stable and has a docs page (REQ-AUT-19).
const (
	CodeParse    = "GX1000"
	CodeFileName = "GX1001"
	CodeStale    = "GX1002"

	CodeRouteField = "GX3000"
	CodePathVar    = "GX3001"
	CodePathField  = "GX3002"
	CodeUnmounted  = "GX3003"
	CodeDuplicate  = "GX3004"
	CodeRoutePkg   = "GX3005"

	CodeRequiredProp     = "GX2001"
	CodeUnknownComponent = "GX2002"
	CodeUnknownAttr      = "GX2003"
	CodeStaticStringProp = "GX2004"

	CodeType = "GX2000"

	CodeSpread        = "GX2005"
	CodeDuplicateSlot = "GX2006"
	CodeEventAttr     = "GX2007"
	CodeFragment      = "GX2008"
	CodeLoopKey       = "GX2009"
	CodeSignal        = "GX2010"
	CodeURLAttr       = "GX2011"
	CodeUnrenderable  = "GX2013"

	CodeImgAlt            = "GX2016"
	CodeControlLabel      = "GX2017"
	CodeNestedInteractive = "GX2018"
	CodeDuplicateID       = "GX2019"
	CodeBadParent         = "GX2020"
	CodeHeadingOrder      = "GX2021"

	CodeTrustedHTML = "GX7001"
	CodeSecret      = "GX7002"

	CodeEnum = "GX5001"

	CodeTransition = "GX5002"

	CodeRuntimeClass = "GX5003"

	CodeContentFrontmatter = "GX8001"

	CodeContentComponent = "GX8002"

	CodeContentLink = "GX8003"

	CodeCodeFile = "GX8004"

	CodeBudget = "GX9001"

	CodeActionMissing       = "GX4001"
	CodeActionTwice         = "GX4002"
	CodeActionMissingSignal = "GX4003"
	CodeSignalTypeMismatch  = "GX4004"
	CodeClientCall          = "GX4005"
	CodeAdapterSignals      = "GX4006"
	CodeClientType          = "GX4007"
	CodeInstanceKey         = "GX2012"
	CodeSignalDefault       = "GX2014"
	CodeSignalRoot          = "GX2015"
	CodeEventMod            = "GX4010"
	// CodeTool is a tool with no description, or with an input field that
	// a JSON value cannot fill (REQ-AI-06, REQ-AI-09).
	CodeTool             = "GX4011"
	CodeUpdateNoFragment = "GX4012"
	CodeOptimistic       = "GX4013"
	CodeSharedSignal     = "GX4014"
	CodeActionMethod     = "GX4009"
	CodeSignalRules      = "GX4008"

	CodeIslandProps = "GX6001"
	CodeIslandType  = "GX6002"
	CodeIslandLoad  = "GX6004"
	// CodeIslandTypeScript is one error of the TypeScript compiler in an
	// island file (REQ-ISL-08).
	CodeIslandTypeScript = "GX6005"
	// CodeWidgetTag is a widget with no tag, with a tag that is not a
	// custom element name, or with the tag of a different widget
	// (REQ-ISL-10).
	CodeWidgetTag = "GX6003"
	// CodeWidgetAttr is a field of a widget input that an attribute
	// cannot hold (REQ-ISL-15).
	CodeWidgetAttr = "GX6006"
	// CodeWidgetHead is gx.Head in the view of a widget, or in a
	// component that the view uses (REQ-ISL-20).
	CodeWidgetHead = "GX6007"
	// CodeWidgetOrigins is a widget, or an action that its component
	// invokes, in a group with no gx.AllowOrigins (REQ-ISL-22).
	CodeWidgetOrigins = "GX6008"
	// CodeCredentialOrigin is a wildcard origin in gx.AllowCredentials
	// (REQ-ISL-22, SI-14).
	CodeCredentialOrigin = "GX6009"

	// CodeEmail is a construct that an email cannot hold (REQ-REG-17).
	CodeEmail = "GX6010"
)

// Diagnostic is one compiler message.
type Diagnostic struct {
	Code string
	File string
	Line int
	Col  int
	Msg  string
	Fix  string
	// Hint is true for a diagnostic that does not fail a check
	// (REQ-ACT-17). Only CheckApp gives hints.
	Hint bool
}

// Failed reports whether a list of diagnostics fails a check: it holds one
// that is not a hint.
func Failed(diags []Diagnostic) bool {
	for _, d := range diags {
		if !d.Hint {
			return true
		}
	}
	return false
}

// Info describes one diagnostic code.
type Info struct {
	Code  string
	Title string
}

// Catalog lists every diagnostic code (REQ-AUT-19). Keep it in step with the
// code constants above; the catalog test fails when it does not.
var Catalog = []Info{
	{CodeParse, "parse error"},
	{CodeFileName, "component file name is not an exported identifier"},
	{CodeStale, "generated code is missing or stale"},
	{CodeType, "type error in an expression"},
	{CodeRequiredProp, "missing required prop"},
	{CodeUnknownComponent, "unknown component"},
	{CodeUnknownAttr, "unknown attribute or slot"},
	{CodeStaticStringProp, "static value for a typed prop"},
	{CodeSpread, "attribute spread on a component"},
	{CodeDuplicateSlot, "duplicate slot"},
	{CodeEventAttr, "dynamic event attribute"},
	{CodeFragment, "fragment free variable"},
	{CodeLoopKey, "loop needs a key"},
	{CodeSignal, "signal in a server expression"},
	{CodeInstanceKey, "signal instance needs a key"},
	{CodeSignalDefault, "signal needs an initial value"},
	{CodeSignalRoot, "component with signals has no top-level HTML element"},
	{CodeImgAlt, "img has no alt attribute"},
	{CodeControlLabel, "form control has no label"},
	{CodeNestedInteractive, "interactive element inside a link or a button"},
	{CodeDuplicateID, "one id on two elements of a component"},
	{CodeBadParent, "element in a parent where a browser moves or drops it"},
	{CodeHeadingOrder, "heading skips a level"},
	{CodeURLAttr, "dynamic URL attribute"},
	{CodeUnrenderable, "value cannot render as text"},
	{CodeRouteField, "route field type cannot bind"},
	{CodePathVar, "pattern variable has no field"},
	{CodePathField, "path field has no pattern variable"},
	{CodeUnmounted, "route value is not held by any gx.Collect"},
	{CodeDuplicate, "duplicate route pattern"},
	{CodeRoutePkg, "route package contents"},
	{CodeActionMissing, "no action is registered for a route type"},
	{CodeActionTwice, "more than one action is registered for a route type"},
	{CodeActionMissingSignal, "no signal is declared for a signal-bound field"},
	{CodeSignalTypeMismatch, "signal type does not match the action field"},
	{CodeClientCall, "call is not allowed in a client expression"},
	{CodeAdapterSignals, "signal or client expression under an adapter with no signals"},
	{CodeClientType, "value or operator is not allowed in a client expression"},
	{CodeSignalRules, "signal-bound fields need rules or gx.Unchecked"},
	{CodeActionMethod, "action method cannot be invoked from the client"},
	{CodeEventMod, "unknown event modifier or special event"},
	{CodeTool, "tool has no description or an input with no JSON form"},
	{CodeUpdateNoFragment, "c.Update takes a component with no fragment"},
	{CodeOptimistic, "optimistic directive has no action or writes no signal"},
	{CodeSharedSignal, "shared signal in a component with no gx.Room prop"},
	{CodeEnum, "gx.Enum misses a constant of its type"},
	{CodeTransition, "duplicate view-transition-name in one template"},
	{CodeRuntimeClass, "class string is built at runtime"},
	{CodeIslandProps, "island has no props struct"},
	{CodeIslandType, "island prop type has no TypeScript mapping"},
	{CodeWidgetTag, "widget tag is missing, not valid or used two times"},
	{CodeIslandLoad, "unknown island load strategy"},
	{CodeIslandTypeScript, "TypeScript error in an island"},
	{CodeWidgetAttr, "widget input field cannot be an attribute"},
	{CodeWidgetHead, "gx.Head in a widget"},
	{CodeWidgetOrigins, "widget route is in a group with no origins"},
	{CodeCredentialOrigin, "origin with cookies is not exact"},
	{CodeEmail, "construct that an email cannot hold"},
	{CodeTrustedHTML, "conversion to gx.SafeHTML needs //gx:trusted"},
	{CodeSecret, "gx.Secret cannot cross to the client"},
	{CodeContentFrontmatter, "frontmatter is malformed or unknown"},
	{CodeContentComponent, "component is not declared in this collection"},
	{CodeContentLink, "content link or anchor is broken"},
	{CodeCodeFile, "code file or line range is missing"},
	{CodeBudget, "page route is over its budget"},
}

// Doc returns the documentation path of the diagnostic (REQ-AUT-19).
func (d Diagnostic) Doc() string { return "/errors/" + d.Code }

func (d Diagnostic) String() string {
	where := d.File
	if where == "" {
		where = "<input>"
	}
	if d.Hint {
		return fmt.Sprintf("%s:%d:%d: %s: hint: %s", where, d.Line, d.Col, d.Code, d.Msg)
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", where, d.Line, d.Col, d.Code, d.Msg)
}

// Quoted returns s as a double-quoted Go string.
func Quoted(s string) string {
	return strconv.Quote(s)
}
