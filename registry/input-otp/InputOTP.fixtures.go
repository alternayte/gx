package inputotp

import "github.com/alternayte/gx"

var InputOTPFixtures = gx.Fixtures[InputOTPProps]{
	"Empty":    {Id: "otp-empty", Name: "code", Label: "One-time code"},
	"Filled":   {Id: "otp-filled", Name: "code", Label: "One-time code", Value: "123"},
	"Grouped":  {Id: "otp-grouped", Name: "code", Label: "One-time code", Group: 3},
	"Four":     {Id: "otp-four", Name: "pin", Label: "PIN", Length: 4},
	"Disabled": {Id: "otp-disabled", Name: "code", Label: "One-time code", Value: "12", Disabled: true},
}
