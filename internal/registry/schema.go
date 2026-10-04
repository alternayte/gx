package registry

import _ "embed"

// itemSchema is the published JSON Schema of one registry item
// (REQ-REG-01).
//
//go:embed schema.json
var itemSchema []byte

// Schema returns the JSON Schema of one registry item (REQ-REG-01).
func Schema() ([]byte, error) { return itemSchema, nil }
