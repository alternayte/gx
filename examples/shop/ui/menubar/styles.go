package menubar

var activeClass = map[bool]string{
	true:  "bg-accent text-accent-foreground",
	false: "text-foreground",
}

// current returns the aria-current value of one item.
func (p MenubarItemProps) current() string {
	if p.Active {
		return "page"
	}
	return ""
}
