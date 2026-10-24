package configtest

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFakeWhoseConfigIsRemovedHasNoneToLoadNorToRememberInto(t *testing.T) {
	fake := NewFake(FakeParams{Config: seeded()})

	fake.Remove()
	_, _, loadErr := fake.Load()
	_, rememberErr := fake.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

	require.ErrorIs(t, loadErr, project.ErrConfigNotFound)
	require.ErrorIs(t, rememberErr, project.ErrConfigNotFound)
}

func TestAFakeLoadsAndLoadsItsRunAsSeededOrFailsAsTold(t *testing.T) {
	refused := errors.New("permission denied")
	run := project.Run{Declared: true, Jobs: []project.Job{{Name: "web", Cmd: "pnpm dev"}}}
	fake := NewFake(FakeParams{Config: seeded(), Run: run})
	failing := NewFake(FakeParams{Config: seeded(), Run: run, Fail: map[string]error{"Load": refused, "LoadRun": refused}})

	loaded, loadErr := fake.LoadRun()
	_, _, failedLoad := failing.Load()
	_, failedRun := failing.LoadRun()

	require.NoError(t, loadErr)
	assert.Equal(t, run, loaded)
	require.ErrorIs(t, failedLoad, refused)
	require.ErrorIs(t, failedRun, refused)
}

func TestAFakeRecordsEachCallInOrder(t *testing.T) {
	fake := NewFake(FakeParams{Config: seeded()})

	_, _, _ = fake.Load()
	_, _ = fake.LoadRun()
	_, _ = fake.Remember(t.Context(), nil)

	assert.Equal(t, []string{"Load", "LoadRun", "Remember"}, fake.Calls())
}

func TestWritingIntoALoadedConfigLeavesTheFakeAsItWas(t *testing.T) {
	seed := seeded()
	fake := NewFake(FakeParams{Config: seed})

	loaded, _, err := fake.Load()
	require.NoError(t, err)
	loaded.Remembered[project.RememberIsolation] = "verbatim"
	loaded.Env.Files[0].Target = "changed"
	loaded.OnCreate[0].Cmd = "changed"
	seed.Env.Files[0].Target = "changed too"
	written, err := fake.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

	require.NoError(t, err)
	assert.Equal(t, domain.Written{Changed: true}, written)
	again, _, err := fake.Load()
	require.NoError(t, err)
	assert.Equal(t, ".env", again.Env.Files[0].Target)
	assert.Equal(t, "make setup", again.OnCreate[0].Cmd)
}
