package repository_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

	"github.com/stretchr/testify/assert"
)

func TestTheLastLineOfEachLogIsReadAndALogThatHoldsNoneIsLeftOut(t *testing.T) {
	var asked []run.LogFile
	read := func(file run.LogFile, _ int) ([]run.Line, error) {
		asked = append(asked, file)
		if file.Job == "migrate" {
			return []run.Line{{Text: "migrating"}, {Text: "migrated"}}, nil
		}
		return nil, nil
	}

	lines := repository.LastLines(repository.LastLinesParams{StateDir: "/state", Branch: "feat/a", Names: []string{"migrate", "web"}, Read: read})

	assert.Equal(t, map[string]*run.Line{"migrate": {Text: "migrated"}}, lines)
	assert.Equal(t, "/state/logs/feat%2Fa", asked[0].Dir)
	assert.Empty(t, repository.LastLines(repository.LastLinesParams{StateDir: "/state", Names: []string{"web"}, Read: read}), "a worktree on no branch keeps no log")
}
