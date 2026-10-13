package gitwt_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type port interface {
	git.Reader
	git.Writer
}

type implementation struct {
	name string
	port port
}

// implementations start alike: main checked out, feature beside it, main on
// the remote.
func implementations(t *testing.T) []implementation {
	t.Helper()
	dir := resolved(t, gittest.InitRepo(t))
	gittest.AddOrigin(t, dir)
	gittest.CreateBranch(t, dir, "feature")
	return []implementation{
		{name: "git", port: git.Repo{Dir: dir, Remote: "origin"}},
		{name: "fake", port: gittest.NewFake(gittest.FakeParams{Main: tempDir(t), Branch: "main", Local: []string{"feature"}, Remote: []string{"main"}})},
	}
}
