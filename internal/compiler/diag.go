package compiler

import (
	"fmt"
	"strconv"
)

// Diagnostic codes. Every code is stable and has a docs page (REQ-AUT-19).
const (
	CodeParse    = "GX1000"
	CodeFileName = "GX1001"

	CodeRequiredProp     = "GX2001"
	CodeUnknownComponent = "GX2002"
	CodeUnknownAttr      = "GX2003"
	CodeStaticStringProp = "GX2004"

	CodeType = "GX2000"

	CodeSpread        = "GX2005"
	CodeDuplicateSlot = "GX2006"
	CodeEventAttr     = "GX2007"
	CodeSignal        = "GX2010"
	CodeURLAttr       = "GX2011"
	CodeUnrenderable  = "GX2013"
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
