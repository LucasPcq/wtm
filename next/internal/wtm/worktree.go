package wtm

type Meta struct {
	Schema       int         `json:"schema,omitempty"`
	SourceBranch string      `json:"source_branch"`
	CreatedAt    string      `json:"created_at"`
	EnvStrategy  EnvStrategy `json:"env_strategy"`
	Ordinal      int         `json:"ordinal,omitempty"`
	Namespaces   []string    `json:"namespaces,omitempty"`
	Isolation    Isolation   `json:"isolation,omitempty"`
}

type Identity struct {
	Branch    string
	Path      string
	Parent    string
	Ordinal   int
	Isolation Isolation
	CreatedAt string
}

type Worktree struct {
	Path     string
	Main     bool
	Locked   bool
	Prunable bool
	State    WorktreeState
}

//go-sumtype:decl WorktreeState

type WorktreeState interface{ worktreeState() }

type OnBranch struct{ Branch string }

type Rebasing struct{ Branch string }

type Detached struct{}

type Bare struct{}

func (OnBranch) worktreeState() {}
func (Rebasing) worktreeState() {}
func (Detached) worktreeState() {}
func (Bare) worktreeState()     {}

//go-sumtype:decl Checkout

type Checkout interface{ checkout() }

type NewBranch struct {
	Name string
	From Ref
}

type ExistingBranch struct{ Name string }

type TrackingBranch struct{ Name string }

func (NewBranch) checkout()      {}
func (ExistingBranch) checkout() {}
func (TrackingBranch) checkout() {}

//go-sumtype:decl Ref

type Ref interface{ ref() }

type LocalBranch struct{ Name string }

type RemoteBranch struct{ Name string }

func (LocalBranch) ref()  {}
func (RemoteBranch) ref() {}

type Branch struct {
	Name string
	Tip  string
}

type AheadBehind struct {
	Ahead  int
	Behind int
}

type Divergence string

const (
	NoOrigin Divergence = ""
	UpToDate Divergence = "up-to-date"
	Behind   Divergence = "behind"
	Ahead    Divergence = "ahead"
	Diverged Divergence = "diverged"
)
