package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWordIsQuotedOnlyWhenTheShellWouldReadItOtherwise(t *testing.T) {
	assert.Equal(t, []string{"feat/a", `'it'\''s'`, "''", "'two words'"}, Quoted([]string{"feat/a", "it's", "", "two words"}))
}

func TestAFollowUpIsItsCommandLineEachWordQuotedForTheShell(t *testing.T) {
	assert.Equal(t, "wtm clean feat/x --force", Line([]string{"wtm", "clean", "feat/x", "--force"}))
	assert.Equal(t, `wtm clean 'it'\''s' 'a$b;c!' --force`, Line([]string{"wtm", "clean", "it's", "a$b;c!", "--force"}))
}

func TestAPathIsShownRelativeToWhereItRuns(t *testing.T) {
	assert.Equal(t, "../.trees/feat-x", ShownPath(Shown{Cwd: "/repo/main", Path: "/repo/.trees/feat-x"}))
	assert.Equal(t, "/repo/.trees/feat-x", ShownPath(Shown{Path: "/repo/.trees/feat-x"}))
}

func TestAWorktreeIsShownByItsBranchElseItsPathFromWhereItRuns(t *testing.T) {
	assert.Equal(t, "feat/a", ShownWorktree("feat/a", Shown{Cwd: "/repo", Path: "/repo/.trees/a"}))
	assert.Equal(t, ".trees/d", ShownWorktree("", Shown{Cwd: "/repo", Path: "/repo/.trees/d"}))
}
