package git

//go-sumtype:decl Ref

type Ref interface{ ref() }

type LocalBranch struct {
	Name string
}

type RemoteBranch struct {
	Name string
}

func (LocalBranch) ref()  {}
func (RemoteBranch) ref() {}

const (
	headsPrefix   = "refs/heads/"
	remotesPrefix = "refs/remotes/"
)

func (r Repo) refName(ref Ref) (string, error) {
	switch branch := ref.(type) {
	case LocalBranch:
		return headsPrefix + branch.Name, nil
	case RemoteBranch:
		return r.remotePrefix() + branch.Name, nil
	}
	return "", errNoRef
}

func (r Repo) remotePrefix() string { return remotesPrefix + r.Remote + "/" }
