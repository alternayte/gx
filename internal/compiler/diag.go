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

	CodeTrustedHTML = "GX7001"
)

// Diagnostic is one compiler message.
type Diagnostic struct {
	Code string
	File string
	Line int
	Col  int
	Msg  string
	Fix  string
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
	{CodeURLAttr, "dynamic URL attribute"},
	{CodeUnrenderable, "value cannot render as text"},
	{CodeRouteField, "route field type cannot bind"},
	{CodePathVar, "pattern variable has no field"},
	{CodePathField, "path field has no pattern variable"},
	{CodeUnmounted, "route value is not held by any gx.Collect"},
	{CodeDuplicate, "duplicate route pattern"},
	{CodeTrustedHTML, "conversion to gx.SafeHTML needs //gx:trusted"},
}

// Doc returns the documentation path of the diagnostic (REQ-AUT-19).
func (d Diagnostic) Doc() string { return "/errors/" + d.Code }

func (d Diagnostic) String() string {
	where := d.File
	if where == "" {
		where = "<input>"
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", where, d.Line, d.Col, d.Code, d.Msg)
}

// Quoted returns s as a double-quoted Go string.
func Quoted(s string) string {
	return strconv.Quote(s)
}
