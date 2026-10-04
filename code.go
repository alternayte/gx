package gx

// Code is one code block of the docs kit (REQ-CNT-05). The compiler fills
// it from gx.CodeFile at build time and fails the build when the file or a
// selected line is missing.
type Code struct {
	// File is the module-relative path of the source file.
	File string
	// Lang is the chroma lexer name; empty guesses from File.
	Lang string
	// Source is the selected source text with a trailing newline.
	Source string
}

// CodeFile names a repository file for a code block (REQ-CNT-05). The
// compiler replaces the call with the resolved gx.Code value. The values
// here keep a generated file type-checking and make the intent explicit.
func CodeFile(path, lines string) Code {
	return Code{File: path}
}
