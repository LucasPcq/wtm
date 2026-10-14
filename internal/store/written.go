package store

// Written is what a write did. Unlocked is a write that ran without its lock,
// on a filesystem without flock: the caller reports it, the write still landed.
type Written struct {
	Changed  bool
	Unlocked bool
}
