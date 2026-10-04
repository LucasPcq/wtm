package domain

// EventType names one kind of message on the `wtm events` stream. Consumers
// ignore a type they do not know: a new one is never a breaking change.
type EventType string

const (
	EventSnapshot              EventType = "snapshot"
	EventReady                 EventType = "ready"
	EventWorktreeCreated       EventType = "worktree.created"
	EventWorktreeProvisioned   EventType = "worktree.provisioned"
	EventWorktreeUpdated       EventType = "worktree.updated"
	EventWorktreeRelocated     EventType = "worktree.relocated"
	EventWorktreeReparented    EventType = "worktree.reparented"
	EventWorktreeDeprovisioned EventType = "worktree.deprovisioned"
	EventWorktreeRemoved       EventType = "worktree.removed"
	EventRepoAdded             EventType = "repo.added"
	EventRepoRemoved           EventType = "repo.removed"

	// EventWorktreePrefix opens every type about one worktree's identity.
	EventWorktreePrefix = "worktree."
)

// EventTypes is every type v1 defines, in the order the guide documents them.
var EventTypes = []EventType{
	EventSnapshot,
	EventReady,
	EventWorktreeCreated,
	EventWorktreeProvisioned,
	EventWorktreeUpdated,
	EventWorktreeRelocated,
	EventWorktreeReparented,
	EventWorktreeDeprovisioned,
	EventWorktreeRemoved,
	EventRepoAdded,
	EventRepoRemoved,
}

// EventsSchemaVersion moves on a breaking change only.
const EventsSchemaVersion = 1

// EventsFinalExitCodes are the codes `wtm events` ends on that a retry cannot
// change; any other non-zero exit is worth retrying.
var EventsFinalExitCodes = []int{
	ExitCodeUsage,
	ExitCodeConfigNotFound,
	ExitCodeEventsSchemaNewer,
	ExitCodeNotGitRepo,
}

type IdentityField string

const (
	IdentityIsolation IdentityField = "isolation"
	IdentityOrdinal   IdentityField = "ordinal"
	IdentityParent    IdentityField = "parent"
	IdentityCreatedAt IdentityField = "created_at"
)

// RegisteredRepo is one line of the registry a global `wtm events` follows.
type RegisteredRepo struct {
	CommonDir string `json:"common_dir"`
	Root      string `json:"root"`
	AddedAt   string `json:"added_at"`
}

const (
	RegistryFileName = "repos.json"
	RegistryLockName = "repos.json.lock"
)

type EventRepo struct {
	Root      string `json:"root"`
	CommonDir string `json:"common_dir"`
}

// WorktreeIdentity is what a consumer keys and converges on: nothing volatile
// (dirty, ahead, PR, services) — those cost a git call each and change without
// any wtm command running.
type WorktreeIdentity struct {
	Branch    string    `json:"branch"`
	Path      string    `json:"path"`
	Parent    string    `json:"parent"`
	Ordinal   *int      `json:"ordinal"`
	Isolation Isolation `json:"isolation"`
	IsMain    bool      `json:"is_main"`
	CreatedAt string    `json:"created_at"`
}

type Event struct {
	V             int                `json:"v"`
	Type          EventType          `json:"type"`
	TS            string             `json:"ts"`
	CorrelationID string             `json:"correlation_id,omitempty"`
	Repo          *EventRepo         `json:"repo,omitempty"`
	Worktrees     []WorktreeIdentity `json:"worktrees,omitempty"`
	Worktree      *WorktreeIdentity  `json:"worktree,omitempty"`
	Changed       []IdentityField    `json:"changed,omitempty"`
	FromPath      string             `json:"from_path,omitempty"`
	FromParent    string             `json:"from_parent,omitempty"`
	OK            *bool              `json:"ok,omitempty"`
	Hook          string             `json:"hook,omitempty"`
	ExitCode      *int               `json:"exit_code,omitempty"`
}
