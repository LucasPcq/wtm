package storetest

import "github.com/LucasPcq/wtm/internal/store"

type Disk struct {
	Meta   store.Meta
	Config store.ProjectConfig
	Env    store.EnvFiles
}

// OnDisk is the real ports on a test's state dir, for a test that must see what a removed worktree takes with it.
func OnDisk(stateDir string) Disk {
	return Disk{Meta: store.MetaDisk{StateDir: stateDir}, Config: store.ConfigDisk{StateDir: stateDir}, Env: store.EnvDisk{}}
}
