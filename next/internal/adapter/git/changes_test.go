package git_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangesListsEachUncommittedFileAsGitOrdersIt(t *testing.T) {
	p := sourceAndTarget(t)
	write(t, filepath.Join(p.source, "a.txt"), "changed\n")
	write(t, filepath.Join(p.source, "c.txt"), "staged\n")
	gittest.Git(t, p.source, "add", "c.txt")
	write(t, filepath.Join(p.source, "c.txt"), "staged then changed\n")
	gittest.Git(t, p.source, "rm", "-q", "gone.txt")
	gittest.Git(t, p.source, "mv", "README.md", "DOC.md")
	write(t, filepath.Join(p.source, "new", "x.txt"), "x\n")
	write(t, filepath.Join(p.source, "new", "deep", "y.txt"), "y\n")
	write(t, filepath.Join(p.source, "with space.txt"), "s\n")
	write(t, filepath.Join(p.source, "ignored.log"), "noise\n")

	changes, err := p.repo.Changes(t.Context(), p.source)

	require.NoError(t, err)
	assert.Equal(t, []worktree.Change{
		{Path: "DOC.md", Status: worktree.ChangeRenamed, OrigPath: "README.md"},
		{Path: "a.txt", Status: worktree.ChangeModified},
		{Path: "c.txt", Status: worktree.ChangeModified},
		{Path: "gone.txt", Status: worktree.ChangeDeleted},
		{Path: "new/deep/y.txt", Status: worktree.ChangeUntracked},
		{Path: "new/x.txt", Status: worktree.ChangeUntracked},
		{Path: "with space.txt", Status: worktree.ChangeUntracked},
	}, changes)
}

func TestNothingConflictsWithATargetThatLeftTheFilesAlone(t *testing.T) {
	p := sourceAndTarget(t)
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "two"))
	write(t, filepath.Join(p.source, "fresh.txt"), "fresh\n")

	conflicts, err := p.repo.Conflicts(t.Context(), p.check(
		worktree.Change{Path: "a.txt", Status: worktree.ChangeModified},
		worktree.Change{Path: "fresh.txt", Status: worktree.ChangeUntracked},
	))

	require.NoError(t, err)
	assert.Empty(t, conflicts)
}

func TestATargetThatCommittedAnotherChangeToTheSameLinesConflicts(t *testing.T) {
	p := sourceAndTarget(t)
	commitFile(t, p.target, "a.txt", replaceLine(numbered, 2, "target's two"))
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "source's two"))

	conflicts, err := p.repo.Conflicts(t.Context(), p.check(worktree.Change{Path: "a.txt", Status: worktree.ChangeModified}))

	require.NoError(t, err)
	assert.Equal(t, []worktree.Conflict{{Path: "a.txt", Kind: worktree.ConflictModified}}, conflicts)
}

func TestATargetThatChangedOtherLinesDoesNotConflict(t *testing.T) {
	p := sourceAndTarget(t)
	commitFile(t, p.target, "a.txt", replaceLine(numbered, 8, "target's eight"))
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "source's two"))

	conflicts, err := p.repo.Conflicts(t.Context(), p.check(worktree.Change{Path: "a.txt", Status: worktree.ChangeModified}))

	require.NoError(t, err)
	assert.Empty(t, conflicts)
}

func TestADeletionConflictsWithATargetThatModifiedTheFile(t *testing.T) {
	p := sourceAndTarget(t)
	write(t, filepath.Join(p.target, "gone.txt"), "kept and changed\n")
	gittest.Git(t, p.source, "rm", "-q", "gone.txt")

	conflicts, err := p.repo.Conflicts(t.Context(), p.check(worktree.Change{Path: "gone.txt", Status: worktree.ChangeDeleted}))

	require.NoError(t, err)
	assert.Equal(t, []worktree.Conflict{{Path: "gone.txt", Kind: worktree.ConflictModified}}, conflicts)
}

func TestAnUntrackedFileCollidesOnlyWithDifferentBytesAndABinaryOneCannotBeMerged(t *testing.T) {
	p := sourceAndTarget(t)
	for name, sides := range map[string][2]string{"same.txt": {"same\n", "same\n"}, "other.txt": {"source\n", "target\n"}, "blob.bin": {"a\x00b", "a\x00c"}} {
		write(t, filepath.Join(p.source, name), sides[0])
		write(t, filepath.Join(p.target, name), sides[1])
	}

	conflicts, err := p.repo.Conflicts(t.Context(), p.check(
		worktree.Change{Path: "blob.bin", Status: worktree.ChangeUntracked},
		worktree.Change{Path: "other.txt", Status: worktree.ChangeUntracked},
		worktree.Change{Path: "same.txt", Status: worktree.ChangeUntracked},
	))

	require.NoError(t, err)
	assert.Equal(t, []worktree.Conflict{{Path: "blob.bin", Kind: worktree.ConflictBinary}, {Path: "other.txt", Kind: worktree.ConflictExists}}, conflicts)
}

func TestATargetNotCreatedYetIsCheckedAgainstItsStartRef(t *testing.T) {
	p := sourceAndTarget(t)
	commitFile(t, p.target, "a.txt", replaceLine(numbered, 2, "target's two"))
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "source's two"))
	changes := []worktree.Change{{Path: "a.txt", Status: worktree.ChangeModified}}

	fromSplit, splitErr := p.repo.Conflicts(t.Context(), worktree.ConflictCheck{Source: p.source, TargetRef: "split", Changes: changes})
	fromMain, mainErr := p.repo.Conflicts(t.Context(), worktree.ConflictCheck{Source: p.source, TargetRef: "main", Changes: changes})
	_, unknownErr := p.repo.Conflicts(t.Context(), worktree.ConflictCheck{Source: p.source, TargetRef: "nowhere", Changes: changes})

	require.NoError(t, splitErr)
	assert.Equal(t, []worktree.Conflict{{Path: "a.txt", Kind: worktree.ConflictModified}}, fromSplit)
	require.NoError(t, mainErr)
	assert.Empty(t, fromMain)
	require.Error(t, unknownErr)
}

func TestAMoveWritesTheTargetThenCleansTheSource(t *testing.T) {
	p := sourceAndTarget(t)
	commitFile(t, p.target, "c.txt", replaceLine(numbered, 8, "target's eight"))
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "two"))
	write(t, filepath.Join(p.source, "c.txt"), replaceLine(numbered, 2, "two"))
	gittest.Git(t, p.source, "rm", "-q", "gone.txt")
	gittest.Git(t, p.source, "mv", "README.md", "DOC.md")
	write(t, filepath.Join(p.source, "new", "deep", "x.txt"), "x\n")
	write(t, filepath.Join(p.source, "left.txt"), "not selected\n")

	moved, err := p.repo.Move(t.Context(), p.move(
		worktree.Change{Path: "DOC.md", Status: worktree.ChangeRenamed, OrigPath: "README.md"},
		worktree.Change{Path: "a.txt", Status: worktree.ChangeModified},
		worktree.Change{Path: "c.txt", Status: worktree.ChangeModified},
		worktree.Change{Path: "gone.txt", Status: worktree.ChangeDeleted},
		worktree.Change{Path: "new/deep/x.txt", Status: worktree.ChangeUntracked},
	))

	require.NoError(t, err)
	assert.Equal(t, worktree.Moved{}, moved)
	assert.Equal(t, []string{"R  README.md -> DOC.md", "M  a.txt", " M c.txt", "D  gone.txt", "?? new/deep/x.txt"}, porcelain(t, p.target))
	assert.Equal(t, replaceLine(replaceLine(numbered, 8, "target's eight"), 2, "two"), contentOf(t, filepath.Join(p.target, "c.txt")))
	assert.Equal(t, []string{"?? left.txt"}, porcelain(t, p.source))
	assert.NoDirExists(t, filepath.Join(p.source, "new"))
}

func TestAKeptMoveLeavesTheSourceAsItWas(t *testing.T) {
	p := sourceAndTarget(t)
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "two"))
	write(t, filepath.Join(p.source, "fresh.txt"), "fresh\n")
	move := p.move(worktree.Change{Path: "a.txt", Status: worktree.ChangeModified}, worktree.Change{Path: "fresh.txt", Status: worktree.ChangeUntracked})
	move.Keep = true

	moved, err := p.repo.Move(t.Context(), move)

	require.NoError(t, err)
	assert.Equal(t, worktree.Moved{Kept: true}, moved)
	assert.Equal(t, []string{"M  a.txt", "?? fresh.txt"}, porcelain(t, p.target))
	assert.Equal(t, []string{" M a.txt", "?? fresh.txt"}, porcelain(t, p.source))
}

func TestAResolvedMoveWritesMarkersInTheTargetAndKeepsTheSource(t *testing.T) {
	p := sourceAndTarget(t)
	commitFile(t, p.target, "a.txt", replaceLine(numbered, 2, "target's two"))
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "source's two"))
	write(t, filepath.Join(p.source, "c.txt"), replaceLine(numbered, 2, "two"))
	move := p.move(worktree.Change{Path: "a.txt", Status: worktree.ChangeModified}, worktree.Change{Path: "c.txt", Status: worktree.ChangeModified})
	move.Resolve = true

	moved, err := p.repo.Move(t.Context(), move)

	require.NoError(t, err)
	assert.Equal(t, worktree.Moved{Merged: []string{"a.txt"}, Kept: true}, moved)
	assert.Contains(t, contentOf(t, filepath.Join(p.target, "a.txt")), "<<<<<<< split\ntarget's two\n=======\nsource's two\n>>>>>>> incoming (main)\n")
	assert.Equal(t, []string{" M a.txt", "M  c.txt"}, porcelain(t, p.target))
	assert.Equal(t, []string{" M a.txt", " M c.txt"}, porcelain(t, p.source))
}

func TestAMoveRefusedForConflictsNamesOnlyThemAndTouchesNothing(t *testing.T) {
	p := sourceAndTarget(t)
	commitFile(t, p.target, "a.txt", replaceLine(numbered, 2, "target's two"))
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "source's two"))
	write(t, filepath.Join(p.source, "c.txt"), replaceLine(numbered, 2, "two"))
	before := [2][]string{porcelain(t, p.source), porcelain(t, p.target)}

	_, err := p.repo.Move(t.Context(), p.move(worktree.Change{Path: "a.txt", Status: worktree.ChangeModified}, worktree.Change{Path: "c.txt", Status: worktree.ChangeModified}))

	refusal, ok := errors.AsType[*worktree.ConflictRefusal](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, []worktree.Conflict{{Path: "a.txt", Kind: worktree.ConflictModified}}, refusal.Conflicts)
	assert.Equal(t, before, [2][]string{porcelain(t, p.source), porcelain(t, p.target)})
	assert.Equal(t, replaceLine(numbered, 2, "target's two"), contentOf(t, filepath.Join(p.target, "a.txt")))
}

func TestAFailedMoveUndoesWhatItWroteAndNothingTheTargetHeld(t *testing.T) {
	p := sourceAndTarget(t)
	commitFile(t, p.target, "c.txt", replaceLine(numbered, 8, "target's eight"))
	write(t, filepath.Join(p.source, "a.txt"), replaceLine(numbered, 2, "two"))
	write(t, filepath.Join(p.source, "c.txt"), replaceLine(numbered, 2, "two"))
	write(t, filepath.Join(p.source, "same.txt"), "same\n")
	write(t, filepath.Join(p.target, "same.txt"), "same\n")
	write(t, filepath.Join(p.source, "blocker", "x.txt"), "x\n")
	write(t, filepath.Join(p.target, "blocker"), "a file where the source has a folder\n")
	before := [2][]string{porcelain(t, p.source), porcelain(t, p.target)}

	_, err := p.repo.Move(t.Context(), p.move(
		worktree.Change{Path: "a.txt", Status: worktree.ChangeModified},
		worktree.Change{Path: "c.txt", Status: worktree.ChangeModified},
		worktree.Change{Path: "same.txt", Status: worktree.ChangeUntracked},
		worktree.Change{Path: "blocker/x.txt", Status: worktree.ChangeUntracked},
	))

	require.ErrorContains(t, err, "copy blocker/x.txt")
	assert.Equal(t, before, [2][]string{porcelain(t, p.source), porcelain(t, p.target)})
	assert.Equal(t, numbered, contentOf(t, filepath.Join(p.target, "a.txt")))
	assert.Equal(t, replaceLine(numbered, 8, "target's eight"), contentOf(t, filepath.Join(p.target, "c.txt")))
	assert.Equal(t, "same\n", contentOf(t, filepath.Join(p.target, "same.txt")))
}
