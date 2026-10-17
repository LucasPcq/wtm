package domain

// Written is what a write of a state file did: whether it changed the file, and whether it wrote without holding the file's lock.
type Written struct {
	Changed  bool
	Unlocked bool
}
