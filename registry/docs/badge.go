package docs

import "github.com/alternayte/gx"

// badgeClass maps a badge variant to its classes (REQ-STY-05).
var badgeClass = gx.Enum[BadgeVariant]{
	BadgeDefault:     "border-transparent bg-primary text-primary-foreground",
	BadgeSecondary:   "border-transparent bg-secondary text-secondary-foreground",
	BadgeDestructive: "border-transparent bg-destructive text-white",
	BadgeOutline:     "text-foreground",
}

// BadgeVariant is the visual style of a docs badge (REQ-CNT-05).
type BadgeVariant string

// The variants of docs.Badge.
const (
	BadgeDefault     BadgeVariant = "default"
	BadgeSecondary   BadgeVariant = "secondary"
	BadgeDestructive BadgeVariant = "destructive"
	BadgeOutline     BadgeVariant = "outline"
)
