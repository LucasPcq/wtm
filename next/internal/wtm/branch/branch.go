package branch

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
