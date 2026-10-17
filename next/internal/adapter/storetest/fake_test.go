package storetest

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFakeWithoutAConfigRefusesToRemember(t *testing.T) {
	fake := NewFake(FakeParams{})

	_, err := fake.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

	require.ErrorIs(t, err, project.ErrConfigNotFound)
}

func TestAFakeRemembersOnTopOfWhatItHoldsAndNeverForgets(t *testing.T) {
	fake := NewFake(FakeParams{Config: &project.Config{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}})

	written, err := fake.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})
	again, againErr := fake.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

	require.NoError(t, err)
	require.NoError(t, againErr)
	assert.Equal(t, domain.Written{Changed: true}, written)
	assert.Equal(t, domain.Written{}, again)
	config, ok := fake.Config()
	require.True(t, ok)
	assert.Equal(t, map[string]string{project.RememberEnvStrategy: "main", project.RememberIsolation: "verbatim"}, config.Remembered)
}

func TestAFakeRecordAtANewerSchemaIsNeverRewritten(t *testing.T) {
	fake := NewFake(FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Schema: 99, SourceBranch: "main"}}})

	_, err := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error {
		m.Ordinal = 1
		return nil
	}})

	var refusal *domain.SchemaNewer
	require.ErrorAs(t, err, &refusal)
	meta, ok := fake.Meta("feat/a")
	require.True(t, ok)
	assert.Equal(t, worktree.Meta{Schema: 99, SourceBranch: "main"}, meta)
}

func TestAFakeCopiesAndEditsTheFilesItHolds(t *testing.T) {
	fake := NewFake(FakeParams{Files: map[string]string{"/main/.env": "PORT=3000\n"}})

	copied, copyErr := fake.Copy(t.Context(), env.CopyRequest{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main"})
	written, editErr := fake.Edit(t.Context(), dotenv.Edit{Path: "/wt/.env", Edit: func(lines []dotenv.Line) []dotenv.Line {
		updated, _ := dotenv.Set(dotenv.SetParams{Lines: lines, Key: "PORT", Value: "3010"})
		return updated
	}})

	require.NoError(t, copyErr)
	require.NoError(t, editErr)
	assert.Equal(t, []string{"/wt/.env"}, copied.Files)
	assert.True(t, written.Changed)
	content, ok := fake.File("/wt/.env")
	require.True(t, ok)
	assert.Equal(t, "PORT=3010\n", content)
}

func TestAFakeReadsTheLinesOfAFileItHoldsAndNoneOfOneItLacks(t *testing.T) {
	fake := NewFake(FakeParams{Files: map[string]string{"/wt/.env": "A=1\n"}})

	lines, err := fake.Read("/wt/.env")
	require.NoError(t, err)
	absent, err := fake.Read("/wt/.env.local")
	require.NoError(t, err)

	assert.Equal(t, "A=1\n", string(dotenv.Render(lines)))
	assert.Nil(t, absent)
	assert.Equal(t, []string{"Read /wt/.env", "Read /wt/.env.local"}, fake.Calls())
}

func TestAFakeUpdateThatChangesNoByteOfTheRecordChangesNothing(t *testing.T) {
	fake := NewFake(FakeParams{Meta: map[string]worktree.Meta{"feat/a": {SourceBranch: "main"}}})

	written, err := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(meta *worktree.Meta) error {
		meta.Namespaces = []string{}
		return nil
	}})

	require.NoError(t, err)
	assert.Equal(t, domain.Written{}, written)
	stored, _ := fake.Meta("feat/a")
	assert.Nil(t, stored.Namespaces)
}

func TestAFakeRecordIsStoredAsItsJSONReadsBack(t *testing.T) {
	fake := NewFake(FakeParams{})

	_, err := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(meta *worktree.Meta) error {
		meta.Namespaces = []string{}
		return nil
	}})

	require.NoError(t, err)
	stored, ok := fake.Meta("feat/a")
	require.True(t, ok)
	assert.Nil(t, stored.Namespaces)
}

func TestAFakeRefusesABranchThatIsNoPathSegmentAsTheStoreDoes(t *testing.T) {
	fake := NewFake(FakeParams{})

	_, loadErr := fake.Load("..")
	_, updateErr := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "..", Change: func(*worktree.Meta) error { return nil }})

	assert.Error(t, loadErr)
	assert.Error(t, updateErr)
}

func TestAFakeConfigAtANewerSchemaOrThatDoesNotParseIsNeverRemembered(t *testing.T) {
	newer := NewFake(FakeParams{Config: &project.Config{}, ConfigSchema: 2})
	broken := NewFake(FakeParams{Config: &project.Config{}, ConfigUnreadable: true})

	_, newerErr := newer.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})
	_, brokenErr := broken.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

	var refusal *domain.SchemaNewer
	assert.ErrorAs(t, newerErr, &refusal)
	assert.Error(t, brokenErr)
	config, _ := newer.Config()
	assert.Empty(t, config.Remembered)
}

func TestAFakeNumbersUnderItsLockAndCanFailToTakeIt(t *testing.T) {
	refused := errors.New("no lock")
	fake := NewFake(FakeParams{})
	failing := NewFake(FakeParams{Fail: map[string]error{"LockOrdinals": refused}})

	held, err := fake.LockOrdinals(t.Context(), func() error {
		_, err := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error { m.Ordinal = 1; return nil }})
		return err
	})
	_, failed := failing.LockOrdinals(t.Context(), func() error { return nil })

	require.NoError(t, err)
	assert.True(t, held)
	assert.Equal(t, []string{"LockOrdinals ", "Update feat/a"}, fake.Calls())
	require.ErrorIs(t, failed, refused)
}
