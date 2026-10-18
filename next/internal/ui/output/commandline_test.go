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

func TestGoStepQuotesItsBranchForTheShell(t *testing.T) {
	assert.Equal(t, "wtm go feat/a", GoStep("feat/a").Command)
	assert.Equal(t, "wtm go 'feat/$x'", GoStep("feat/$x").Command)
}

func TestAPathIsShownRelativeToWhereItRuns(t *testing.T) {
	assert.Equal(t, "../.trees/feat-x", ShownPath(Shown{Cwd: "/repo/main", Path: "/repo/.trees/feat-x"}))
	assert.Equal(t, "/repo/.trees/feat-x", ShownPath(Shown{Path: "/repo/.trees/feat-x"}))
}
