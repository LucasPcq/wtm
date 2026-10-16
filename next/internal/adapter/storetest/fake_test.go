package storetest

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/envcopy"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFakeMethodGivenAnErrorFailsWithoutRunning(t *testing.T) {
	full := errors.New("disk full")
	fake := NewFake(FakeParams{
		Config: &project.Config{},
		Fail:   map[string]error{"Update": full, "Remember": full, "Edit": full},
	})

	_, updateErr := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Missing: worktree.MissingCreate, Change: func(*worktree.Meta) error { return nil }})
	_, rememberErr := fake.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})
	_, editErr := fake.Edit(t.Context(), dotenv.Edit{Path: "/wt/.env", Create: true, Edit: func(lines []dotenv.Line) []dotenv.Line { return lines }})

	require.ErrorIs(t, updateErr, full)
	require.ErrorIs(t, rememberErr, full)
	require.ErrorIs(t, editErr, full)
	records, err := fake.Records()
	require.NoError(t, err)
	assert.Empty(t, records)
	config, ok := fake.Config()
	require.True(t, ok)
	assert.Empty(t, config.Remembered)
	_, exists := fake.File("/wt/.env")
	assert.False(t, exists)
}

func TestAFakeRecordsEveryCallInOrder(t *testing.T) {
	fake := NewFake(FakeParams{})

	_, _ = fake.Load("feat/a")
	require.NoError(t, fake.Purge("feat/a"))
	_, err := fake.Copy(t.Context(), envcopy.Request{Strategy: project.EnvMain, TargetDir: "/wt"})
	require.NoError(t, err)

	assert.Equal(t, []string{"Load feat/a", "Purge feat/a", "Copy /wt"}, fake.Calls())
}

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
	assert.Equal(t, wtm.Written{Changed: true}, written)
	assert.Equal(t, wtm.Written{}, again)
	config, ok := fake.Config()
	require.True(t, ok)
	assert.Equal(t, map[string]string{project.RememberEnvStrategy: "main", project.RememberIsolation: "verbatim"}, config.Remembered)
}

func TestAFakeRecordThatDoesNotReadIsLeftOutOfTheRecords(t *testing.T) {
	fake := NewFake(FakeParams{
		Meta:       map[string]worktree.Meta{"feat/a": {Ordinal: 1}},
		Unreadable: []string{"feat/a"},
	})

	records, err := fake.Records()

	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestAFakeRecordAtANewerSchemaIsNeverRewritten(t *testing.T) {
	fake := NewFake(FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Schema: 99, SourceBranch: "main"}}})

	_, err := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error {
		m.Ordinal = 1
		return nil
	}})

	require.ErrorIs(t, err, wtm.ErrSchemaNewer)
	meta, ok := fake.Meta("feat/a")
	require.True(t, ok)
	assert.Equal(t, worktree.Meta{Schema: 99, SourceBranch: "main"}, meta)
}

func TestAFakeCopiesAndEditsTheFilesItHolds(t *testing.T) {
	fake := NewFake(FakeParams{Files: map[string]string{"/main/.env": "PORT=3000\n"}})

	copied, copyErr := fake.Copy(t.Context(), envcopy.Request{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main"})
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
	assert.Equal(t, wtm.Written{}, written)
	stored, _ := fake.Meta("feat/a")
	assert.Nil(t, stored.Namespaces)
}

func TestAFakeRecordIsStoredAsItsJSONReadsBack(t *testing.T) {
	fake := NewFake(FakeParams{})

	_, err := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Missing: worktree.MissingCreate, Change: func(meta *worktree.Meta) error {
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
	_, updateErr := fake.Update(t.Context(), worktree.MetaUpdate{Branch: "..", Missing: worktree.MissingCreate, Change: func(*worktree.Meta) error { return nil }})

	assert.Error(t, loadErr)
	assert.Error(t, updateErr)
	assert.NoError(t, fake.Purge(".."))
	records, err := fake.Records()
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestAFakeConfigAtANewerSchemaOrThatDoesNotParseIsNeverRemembered(t *testing.T) {
	newer := NewFake(FakeParams{Config: &project.Config{}, ConfigSchema: 2})
	broken := NewFake(FakeParams{Config: &project.Config{}, ConfigUnreadable: true})

	_, newerErr := newer.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})
	_, brokenErr := broken.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

	var refusal *wtm.SchemaNewer
	assert.ErrorAs(t, newerErr, &refusal)
	assert.Error(t, brokenErr)
	config, _ := newer.Config()
	assert.Empty(t, config.Remembered)
}
