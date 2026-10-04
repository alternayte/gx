package sidebar

var activeClass = map[bool]string{
	true:  "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
	false: "text-sidebar-foreground",
}

// current returns the aria-current value of one item.
func (p SidebarItemProps) current() string {
	if p.Active {
		return "page"
	}
	return ""
}
