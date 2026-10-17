// Package store is the one writer of each state file: a worktree's meta.json, the changes to config.toml and the .env files wtm manages.
package store

// Written is what a write did; Unlocked is a write that landed without its lock, on a filesystem without flock.
type Written struct {
	Changed  bool
	Unlocked bool
}
