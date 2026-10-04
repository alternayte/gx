package gx

// Enum[T] is a class map for the constants of T. The analyzer requires an
// entry for every constant of T (REQ-STY-05).
type Enum[T comparable] map[T]string
