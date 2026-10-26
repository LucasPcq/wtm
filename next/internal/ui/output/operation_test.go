package output

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
)

func TestEveryOperationGitStopsInHasItsWord(t *testing.T) {
	for _, operation := range worktree.Operations() {
		assert.NotEmpty(t, OperationWord(operation), operation)
	}
	assert.Empty(t, OperationWord(""))
}
