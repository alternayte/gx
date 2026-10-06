package command

import "github.com/alternayte/gx"

var CommandFixtures = gx.Fixtures[CommandProps]{
	"Default": {Id: "command-default", Groups: []Group{
		{Heading: "Suggestions", Items: []Item{
			{Label: "Calendar", Keywords: "date day"},
			{Label: "Search emoji", Keywords: "smile"},
			{Label: "Calculator", Disabled: true},
		}},
		{Heading: "Settings", Items: []Item{
			{Label: "Profile", Shortcut: "⌘P", Href: "#profile"},
			{Label: "Billing", Shortcut: "⌘B", Href: "#billing"},
			{Label: "Settings", Shortcut: "⌘S", Value: "settings"},
		}},
	}},
	"OneGroup": {Id: "command-one", Label: "Go to", Placeholder: "Search pages...", Groups: []Group{
		{Heading: "Pages", Items: []Item{
			{Label: "Home", Href: "#home"},
			{Label: "About", Href: "#about"},
		}},
	}},
}
