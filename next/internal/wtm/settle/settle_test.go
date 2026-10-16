package settle

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALinkedWorktreeGetsItsPortsAndItsComposeProject(t *testing.T) {
	writes := resolvedWrites(t, linkedResolve())

	applied := applyWrites(writes, map[string]string{".env": "PORT=3000\n"})

	assert.False(t, writes.Empty())
	assert.Equal(t, []string{".env", "infra/.env"}, writes.Files())
	assert.Equal(t, "PORT=3020\n", applied.Files[".env"])
	assert.Equal(t, "COMPOSE_PROJECT_NAME=shop-feat-a", applied.Files["infra/.env"])
	assert.Equal(t, 20, applied.Plan.Offset)
	assert.Equal(t, []OwnedEntry{{File: "infra/.env", Key: "COMPOSE_PROJECT_NAME", Value: "shop-feat-a", Changed: true}}, applied.Plan.Owned)
}

func TestAVerbatimWorktreeGetsNothingWritten(t *testing.T) {
	params := linkedResolve()
	params.Isolation = run.Verbatim

	writes := resolvedWrites(t, params)

	assert.True(t, writes.Empty())
	assert.Empty(t, writes.Files())
	assert.Equal(t, Writes{}, writes)
}

func TestAProjectWithNoLinkAndNoComposeStackWritesNothing(t *testing.T) {
	params := linkedResolve()
	params.Run = run.Config{Jobs: shopRun().Jobs[:1]}

	writes := resolvedWrites(t, params)

	assert.True(t, writes.Empty())
	assert.Empty(t, writes.Files())
}

func TestAProjectWithOnlyAComposeStackWritesItsNameAlone(t *testing.T) {
	params := linkedResolve()
	params.Run.EnvPorts = nil

	writes := resolvedWrites(t, params)
	applied := applyWrites(writes, nil)

	assert.Equal(t, []string{"infra/.env"}, writes.Files())
	assert.Empty(t, applied.Plan.Entries)
	assert.Equal(t, []OwnedEntry{{File: "infra/.env", Key: "COMPOSE_PROJECT_NAME", Value: "shop-feat-a", Changed: true}}, applied.Plan.Owned)
}

func TestALinkToAFileNotDeclaredIsLeftOut(t *testing.T) {
	params := linkedResolve()
	params.Files = []project.EnvFile{{Target: "infra/.env"}}

	writes := resolvedWrites(t, params)

	assert.Equal(t, []string{"infra/.env"}, writes.Files())
}

func TestApplyingWritesThePortsAndTheKeysWtmOwnsAndSaysWhatChanged(t *testing.T) {
	applied := applyWrites(resolvedWrites(t, linkedResolve()), map[string]string{".env": "# web\nPORT=3000\nDEBUG=1\n"})

	assert.Equal(t, "# web\nPORT=3020\nDEBUG=1\n", applied.Files[".env"])
	assert.Equal(t, "COMPOSE_PROJECT_NAME=shop-feat-a", applied.Files["infra/.env"])
	assert.Equal(t, []string{".env", "infra/.env"}, applied.Changed)
	assert.True(t, applied.Plan.Applied)
	require.Len(t, applied.Plan.Entries, 1)
	assert.Equal(t, StatusRewrite, applied.Plan.Entries[0].Status)
	assert.Equal(t, []OwnedEntry{{File: "infra/.env", Key: "COMPOSE_PROJECT_NAME", Value: "shop-feat-a", Changed: true}}, applied.Plan.Owned)
}

func TestTheSettledPlanReadsTheLinesItIsGivenNotAnEarlierRead(t *testing.T) {
	writes := resolvedWrites(t, linkedResolve())

	applied := applyWrites(writes, map[string]string{".env": "PORT=3020\nADDED=since\n"})

	assert.Equal(t, "PORT=3020\nADDED=since\n", applied.Files[".env"])
	assert.Equal(t, []string{"infra/.env"}, applied.Changed)
	require.Len(t, applied.Plan.Entries, 1)
	assert.Equal(t, StatusUnchanged, applied.Plan.Entries[0].Status)
}

func TestApplyingTwiceChangesNothingTheSecondTime(t *testing.T) {
	writes := resolvedWrites(t, linkedResolve())
	first := applyWrites(writes, map[string]string{".env": "PORT=3000\n"})

	second := applyWrites(writes, first.Files)

	assert.Empty(t, second.Changed)
	require.Len(t, second.Plan.Owned, 1)
	assert.False(t, second.Plan.Owned[0].Changed)
}

func TestAnAbsentLinkedFileStaysAbsentAndItsKeyIsReportedMissing(t *testing.T) {
	writes := resolvedWrites(t, linkedResolve())

	applied := applyWrites(writes, nil)

	assert.False(t, writes.Creates(".env"))
	assert.True(t, writes.Creates("infra/.env"))
	assert.NotContains(t, applied.Files, ".env")
	require.Len(t, applied.Plan.Entries, 1)
	assert.Equal(t, StatusMissingKey, applied.Plan.Entries[0].Status)
}

func TestEmptyWritesSettleIntoNoPlan(t *testing.T) {
	assert.Equal(t, Plan{}, Writes{}.Settled(nil))
}

func TestAValueLinkOnTheComposeProjectWinsOverTheDerivedName(t *testing.T) {
	params := linkedResolve()
	params.Run.EnvValues = []run.EnvValueLink{{File: "infra/.env", Key: "COMPOSE_PROJECT_NAME", Job: "infra", Value: "custom-{worktree}"}}

	applied := applyWrites(resolvedWrites(t, params), map[string]string{".env": "PORT=3000\n"})

	assert.Equal(t, "COMPOSE_PROJECT_NAME=custom-feat-a", applied.Files["infra/.env"])
}

func TestOnlyALinkedWorktreesLinksNeedItsOrdinal(t *testing.T) {
	ports := run.Config{EnvPorts: []run.EnvPortLink{{File: ".env", Key: "PORT"}}}
	values := run.Config{EnvValues: []run.EnvValueLink{{File: ".env", Key: "REALM"}}}

	assert.True(t, NeedsOrdinal(NeedsOrdinalParams{Run: ports}))
	assert.True(t, NeedsOrdinal(NeedsOrdinalParams{Isolation: run.Isolated, Run: values}))
	assert.False(t, NeedsOrdinal(NeedsOrdinalParams{Run: run.Config{}}))
	assert.False(t, NeedsOrdinal(NeedsOrdinalParams{Isolation: run.Verbatim, Run: ports}))
}

func TestAProxyPortOutOfRangeIsRefusedOnlyWhenLinksNeedIt(t *testing.T) {
	params := linkedResolve()
	params.Proxy = project.Proxy{Port: 70000}

	_, err := Resolve(params)
	assert.EqualError(t, err, "invalid [proxy] port 70000 in config.toml — use 1-65535, or leave it out for 11080")

	params.Run.EnvPorts = nil
	_, err = Resolve(params)
	assert.NoError(t, err)
}

func TestAnEnvValueThatCannotBeExpandedIsAnErrorOfRunToml(t *testing.T) {
	params := linkedResolve()
	params.Run.EnvValues = []run.EnvValueLink{{File: ".env", Key: "API", Job: "ghost", Value: "x"}}
	params.Proxy = project.Proxy{Port: 70000}
	broken := linkedResolve()
	broken.Run.EnvValues = params.Run.EnvValues

	_, proxyErr := Resolve(params)
	_, valueErr := Resolve(broken)

	_, fromProxy := errors.AsType[*ValueError](proxyErr)
	value, fromValue := errors.AsType[*ValueError](valueErr)
	require.Error(t, proxyErr)
	assert.False(t, fromProxy)
	require.True(t, fromValue)
	assert.Equal(t, `env API in .env: no job named "ghost"`, value.Error())
}
