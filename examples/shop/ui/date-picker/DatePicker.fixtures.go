package datepicker

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/calendar"
)

var DatePickerFixtures = gx.Fixtures[DatePickerProps]{
	"Empty":   {Id: "date-empty", Name: "day"},
	"Chosen":  {Id: "date-chosen", Name: "day", Value: "2026-10-14"},
	"Limited": {Id: "date-limited", Name: "day", Value: "2026-10-14", Min: "2026-10-10", Max: "2026-10-20", WeekStart: calendar.Monday},
}
