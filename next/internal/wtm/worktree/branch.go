package worktree

type Branch struct {
	Name string
	Tip  string
}

//go-sumtype:decl Ref

type Ref interface{ ref() }

type LocalBranch struct{ Name string }

type RemoteBranch struct{ Name string }

func (LocalBranch) ref()  {}
func (RemoteBranch) ref() {}

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

type Branches struct {
	Local  []Branch
	Remote []Branch
}

func (ab AheadBehind) Divergence() Divergence {
	switch {
	case ab.Ahead > 0 && ab.Behind > 0:
		return Diverged
	case ab.Behind > 0:
		return Behind
	case ab.Ahead > 0:
		return Ahead
	}
	return UpToDate
}
