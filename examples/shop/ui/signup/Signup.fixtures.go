package signup

import "github.com/alternayte/gx"

var SignupFixtures = gx.Fixtures[SignupProps]{
	"Default": {Title: "Create an account", Description: "Start with your email."},
}
