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
	Destructive: "bg-card text-destructive *:data-[slot=alert-description]:text-destructive/90 [&>svg]:text-current",
}

// class returns the classes of one alert. The icon column opens when the
// alert holds an svg as a direct child.
func (p AlertProps) class() string {
	const base = "relative grid w-full grid-cols-[0_1fr] items-start gap-y-0.5 rounded-lg border border-border px-4 py-3 text-sm has-[>svg]:grid-cols-[calc(var(--spacing)*4)_1fr] has-[>svg]:gap-x-3 [&>svg]:size-4 [&>svg]:translate-y-0.5 [&>svg]:text-current"
	variant := p.Variant
	if variant == "" {
		variant = Default
	}
	return gx.Cx(base, variantClass[variant], p.Class)
}
