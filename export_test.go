package gx

import "time"

// ElementByID gives the tests of the package the lookup that a form
// re-render uses (REQ-FRM-05).
func ElementByID(n Node, id string) Node { return findElementByID(n, id) }

// RoomTokenAt gives the tests a token of a room with a given expiry
// (SI-17).
func RoomTokenAt(base, key string, expiry time.Time) Room {
	return Room(roomTokenAt(base, key, expiry.Unix()))
}
