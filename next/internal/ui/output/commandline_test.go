package output

import (
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestAWordIsQuotedOnlyWhenTheShellWouldReadItOtherwise(t *testing.T) {
	assert.Equal(t, []string{"feat/a", `'it'\''s'`, "''", "'two words'"}, Quoted([]string{"feat/a", "it's", "", "two words"}))
	assert.Nil(t, FlagWords(FlagValue{Name: "from"}))
	assert.Equal(t, []string{"--from", "main"}, FlagWords(FlagValue{Name: "from", Value: "main"}))
}

func TestAFollowUpIsTheCommandItNamesWithItsBranchesFirst(t *testing.T) {
	line := FollowUpLine(&domain.FollowUp{Command: "clean", Request: json.RawMessage(`{"branches":["feat/x"],"force":true}`)})

	assert.Equal(t, "wtm clean feat/x --force", line)
}

func TestAPathIsShownRelativeToWhereItRuns(t *testing.T) {
	assert.Equal(t, "../.trees/feat-x", ShownPath(Shown{Cwd: "/repo/main", Path: "/repo/.trees/feat-x"}))
	assert.Equal(t, "/repo/.trees/feat-x", ShownPath(Shown{Path: "/repo/.trees/feat-x"}))
}
