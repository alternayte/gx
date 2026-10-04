package skeleton

import "github.com/alternayte/gx"

var SkeletonFixtures = gx.Fixtures[SkeletonProps]{
	"Line":   {Class: "h-4 w-40"},
	"Circle": {Class: "size-10 rounded-full"},
	"Card":   {Class: "h-24 w-60"},
}
