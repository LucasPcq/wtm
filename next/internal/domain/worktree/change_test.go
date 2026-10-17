package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestARequestedPathSelectsTheChangeItNamesFirst(t *testing.T) {
	available := []Change{{Path: "api"}, {Path: "api/a.go"}, {Path: "web/b.go"}}

	selected, unmatched := SelectChanges(available, []string{"api"})

	assert.Equal(t, []Change{{Path: "api"}}, selected)
	assert.Empty(t, unmatched)
}

func TestARequestedDirectorySelectsEveryChangeUnderItWithOrWithoutItsSlash(t *testing.T) {
	available := []Change{{Path: "api/a.go"}, {Path: "apix.go"}, {Path: "api/new/b.go"}, {Path: "web/c.go"}}

	bare, _ := SelectChanges(available, []string{"api"})
	slashed, _ := SelectChanges(available, []string{"api/"})

	assert.Equal(t, []Change{{Path: "api/a.go"}, {Path: "api/new/b.go"}}, bare)
	assert.Equal(t, bare, slashed)
}

func TestASelectionKeepsEachChangeOnceInTheOrderItWasAskedFor(t *testing.T) {
	available := []Change{{Path: "api/a.go"}, {Path: "web/c.go"}, {Path: "api/b.go"}}

	selected, unmatched := SelectChanges(available, []string{"web/c.go", "api/b.go", "api"})

	assert.Equal(t, []Change{{Path: "web/c.go"}, {Path: "api/b.go"}, {Path: "api/a.go"}}, selected)
	assert.Empty(t, unmatched)
}

func TestASelectionNamesThePathThatMatchedNothing(t *testing.T) {
	available := []Change{{Path: "DOC.md", OrigPath: "README.md", Status: ChangeRenamed}}

	selected, unmatched := SelectChanges(available, []string{"DOC.md", "README.md"})

	assert.Nil(t, selected)
	assert.Equal(t, "README.md", unmatched)
}

func TestAChangeIsTaggedByItsStatusAndARenameLabelledWithBothPaths(t *testing.T) {
	assert.Equal(t, []string{"mod", "new", "del", "ren"}, []string{ChangeModified.Tag(), ChangeUntracked.Tag(), ChangeDeleted.Tag(), ChangeRenamed.Tag()})
	assert.Equal(t, "a.go", Change{Path: "a.go"}.Label())
	assert.Equal(t, "README.md → DOC.md", Change{Path: "DOC.md", OrigPath: "README.md"}.Label())
}
