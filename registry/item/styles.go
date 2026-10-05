package item

import "github.com/alternayte/gx"

// Variant is the surface of an item.
type Variant string

// The variants of item.Item.
const (
	Default Variant = "default"
	Outline Variant = "outline"
	Muted   Variant = "muted"
)

var variantClass = gx.Enum[Variant]{
	Default: "bg-transparent",
	Outline: "border-border",
	Muted:   "bg-muted/50",
}

// Size is the padding and the gap of an item.
type Size string

// The sizes of item.Item.
const (
	Md Size = "default"
	Sm Size = "sm"
)

var sizeClass = gx.Enum[Size]{
	Md: "gap-4 p-4",
	Sm: "gap-2.5 px-4 py-3",
}

// Media is the look of the media slot of an item.
type Media string

// The variants of item.ItemMedia.
const (
	MediaDefault Media = "default"
	MediaIcon    Media = "icon"
	MediaImage   Media = "image"
)

var mediaClass = gx.Enum[Media]{
	MediaDefault: "bg-transparent",
	MediaIcon:    "size-8 rounded-sm border border-border bg-muted [&_svg:not([class*='size-'])]:size-4",
	MediaImage:   "size-10 overflow-hidden rounded-sm [&_img]:size-full [&_img]:object-cover",
}

// variant returns the data-variant value; a zero value is Default.
func (p ItemProps) variant() string {
	if p.Variant == "" {
		return string(Default)
	}
	return string(p.Variant)
}

// size returns the data-size value; a zero value is Md.
func (p ItemProps) size() string {
	if p.Size == "" {
		return string(Md)
	}
	return string(p.Size)
}

// class returns the classes of one item.
func (p ItemProps) class() string {
	const base = "group/item flex flex-wrap items-center rounded-md border border-transparent text-sm transition-colors duration-100 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 [a]:transition-colors [a]:hover:bg-accent/50"
	return gx.Cx(base, variantClass[Variant(p.variant())], sizeClass[Size(p.size())], p.Class)
}

// variant returns the data-variant value; a zero value is MediaDefault.
func (p ItemMediaProps) variant() string {
	if p.Variant == "" {
		return string(MediaDefault)
	}
	return string(p.Variant)
}
