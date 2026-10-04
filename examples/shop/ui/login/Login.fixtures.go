package login

import "github.com/alternayte/gx"

var LoginFixtures = gx.Fixtures[LoginProps]{
	"Default": {Title: "Sign in", Description: "Enter your email and password."},
}
