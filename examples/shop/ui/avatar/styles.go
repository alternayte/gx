package avatar

// Size is the diameter of an avatar.
type Size string

// The sizes of avatar.Avatar.
const (
	Sm Size = "sm"
	Md Size = "default"
	Lg Size = "lg"
)

// size returns the data-size value; a zero value is Md. The size classes
// of the avatar, its fallback and its badge follow this attribute.
func (p AvatarProps) size() string {
	if p.Size == "" {
		return string(Md)
	}
	return string(p.Size)
}
