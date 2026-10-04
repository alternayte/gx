package alert

import "github.com/alternayte/gx"

// Variant is the tone of an alert.
type Variant string

// The variants of alert.Alert.
const (
	Default     Variant = "default"
	Destructive Variant = "destructive"
)

var variantClass = gx.Enum[Variant]{
	Default:     "bg-card text-card-foreground",
	Destructive: "bg-card text-destructive [&>svg]:text-current",
}

// class returns the classes of one alert.
func (p AlertProps) class() string {
	const base = "relative grid w-full grid-cols-[0_1fr] items-start gap-y-0.5 rounded-lg border border-border px-4 py-3 text-sm has-[>svg]:grid-cols-[calc(var(--spacing)*4)_1fr] has-[>svg]:gap-x-3 [&>svg]:size-4 [&>svg]:translate-y-0.5 [&>svg]:text-current"
	grid := "grid-cols-[0_1fr]"
	if p.Icon != nil {
		grid = "grid-cols-[calc(var(--spacing)*4)_1fr] gap-x-3"
	}
	variant := p.Variant
	if variant == "" {
		variant = Default
	}
	return gx.Cx(base, grid, variantClass[variant], p.Class)
}

// descClass returns the description colour of one variant.
func (p AlertProps) descClass() string {
	if p.Variant == Destructive {
		return "text-destructive/90"
	}
	return ""
}
