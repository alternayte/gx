package calendar

import "github.com/alternayte/gx"

var CalendarFixtures = gx.Fixtures[CalendarProps]{
	"Chosen":  {Id: "cal-chosen", Name: "day", Value: "2026-10-14", Label: "Day"},
	"Empty":   {Id: "cal-empty", Name: "day", Month: "2026-02", Label: "Day"},
	"Monday":  {Id: "cal-monday", Name: "day", Value: "2026-10-14", WeekStart: Monday, Label: "Day"},
	"Limited": {Id: "cal-limited", Name: "day", Value: "2026-10-14", Min: "2026-10-10", Max: "2026-10-20", Label: "Day"},
	"German":  {Id: "cal-german", Name: "day", Value: "2026-10-14", WeekStart: Monday, Locale: "de", Label: "Tag"},
}
