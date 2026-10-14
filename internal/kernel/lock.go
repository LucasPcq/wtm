package kernel

type LockScope string

const (
	LockRepo     LockScope = "repo"
	LockWorktree LockScope = "wt"
)

type LockMode string

const (
	LockShared    LockMode = "shared"
	LockExclusive LockMode = "exclusive"
)

type LockKey struct {
	Scope LockScope `json:"scope"`
	Name  string    `json:"name,omitempty"`
	Mode  LockMode  `json:"mode"`
}
