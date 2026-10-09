package docs

import "github.com/alternayte/gx"

// buttonClass maps a link-button variant to its classes (REQ-STY-05).
var buttonClass = gx.Enum[ButtonVariant]{
	ButtonPrimary:   "bg-primary text-primary-foreground hover:bg-primary/90",
	ButtonSecondary: "bg-secondary text-secondary-foreground hover:bg-secondary/80",
	ButtonOutline:   "border border-border bg-transparent hover:bg-accent",
	ButtonGhost:     "bg-transparent hover:bg-accent",
	ButtonMinimal:   "bg-transparent px-0 hover:underline",
}

// ButtonVariant is the visual style of a docs link button (REQ-CNT-05).
type ButtonVariant string

// The variants of docs.LinkButton.
const (
	ButtonPrimary   ButtonVariant = "primary"
	ButtonSecondary ButtonVariant = "secondary"
	ButtonOutline   ButtonVariant = "outline"
	ButtonGhost     ButtonVariant = "ghost"
	ButtonMinimal   ButtonVariant = "minimal"
)

// linkButtonIcon renders the icon after the label of a link button. An
// empty or unknown name renders nothing.
func linkButtonIcon(name IconName) gx.Node {
	body := IconBody(name)
	if body == "" {
		return gx.Text("")
	}
	return gx.Icon(string(body), gx.IconProps{Class: "gx-link-button-icon size-5 shrink-0"})
}
