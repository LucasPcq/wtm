package runenv_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/runenv"
)

func TestALinkedWorktreeGetsItsPortsAndItsComposeProject(t *testing.T) {
	params := linked(t)

	writes, err := runenv.Env(params)

	require.NoError(t, err)
	assert.Equal(t, params.WorktreePath, writes.WorktreePath)
	assert.Equal(t, params.Run.EnvPorts, writes.Links)
	assert.Equal(t, 20, writes.Offset)
	assert.Equal(t, domain.PortOffsetBlock, writes.Block)
	assert.Equal(t, map[domain.PortRef]int{{Job: "web", Name: "http"}: 3000}, writes.Bases)
	assert.Equal(t, []domain.EnvOwnedEntry{{File: "infra/.env", Key: domain.EnvComposeProjectName, Value: "shop-feat-a"}}, writes.Owned)
	assert.Equal(t, "feat-a", writes.Origins.Worktree)
}

func TestAVerbatimWorktreeGetsNothingWritten(t *testing.T) {
	params := linked(t)
	params.Identity.Isolation = domain.IsolationVerbatim

	writes, err := runenv.Env(params)

	require.NoError(t, err)
	assert.True(t, writes.Empty())
	assert.Equal(t, runenv.EnvWrites{}, writes)
}

func TestAProjectWithNoLinkAndNoComposeStackWritesNothing(t *testing.T) {
	params := linked(t)
	params.Run = domain.RunConfig{Jobs: runConfig().Jobs[:1]}

	writes, err := runenv.Env(params)

	require.NoError(t, err)
	assert.True(t, writes.Empty())
}

func TestAProjectWithOnlyAComposeStackWritesItsNameAlone(t *testing.T) {
	params := linked(t)
	params.Run.EnvPorts = nil

	writes, err := runenv.Env(params)

	require.NoError(t, err)
	assert.Empty(t, writes.Links)
	assert.Equal(t, []domain.EnvOwnedEntry{{File: "infra/.env", Key: domain.EnvComposeProjectName, Value: "shop-feat-a"}}, writes.Owned)
}

func TestALinkToAFileNotDeclaredIsLeftOut(t *testing.T) {
	params := linked(t)
	params.EnvFiles = []domain.EnvFile{{Target: "infra/.env"}}

	writes, err := runenv.Env(params)

	require.NoError(t, err)
	assert.Empty(t, writes.Links)
}

func TestAProxyPortOutOfRangeIsRefused(t *testing.T) {
	params := linked(t)
	params.Proxy = domain.ProxyConfig{Port: 70000}

	_, err := runenv.Env(params)

	require.Error(t, err)
}

func TestTheMainCheckoutKeepsItsOwnComposeProjectAndPlainEnvKeys(t *testing.T) {
	params := linked(t)
	params.Run.EnvValues = []domain.EnvValueLink{{File: ".env", Key: "DATABASE", Job: "web"}}
	params.Identity = runenv.Identity{Branch: "chore/dev", IsMain: true}
	params.WorktreePath = params.ProjectDir
	writeFile(t, filepath.Join(params.ProjectDir, "infra", domain.EnvFileName), "COMPOSE_PROJECT_NAME=stack\n")

	writes, err := runenv.Env(params)

	require.NoError(t, err)
	assert.Nil(t, writes.ValueLinks)
	assert.Equal(t, 0, writes.Offset)
	assert.Contains(t, writes.Owned, domain.EnvOwnedEntry{File: "infra/.env", Key: domain.EnvComposeProjectName, Value: "stack"})
}

func TestAValueStampedIntoTheMainCheckoutGoesBackToItsTemplate(t *testing.T) {
	params := linked(t)
	params.Run.EnvValues = []domain.EnvValueLink{{File: ".env", Key: "DATABASE", Job: "web", Value: "stamped"}}
	params.EnvFiles = []domain.EnvFile{{Target: ".env", Template: ".env.template"}}
	params.Identity = runenv.Identity{Branch: "main", IsMain: true}
	params.WorktreePath = params.ProjectDir
	writeFile(t, filepath.Join(params.ProjectDir, ".env"), "DATABASE=stamped\n")
	writeFile(t, filepath.Join(params.ProjectDir, ".env.template"), "DATABASE=app\n")

	writes, err := runenv.Env(params)

	require.NoError(t, err)
	assert.Equal(t, []domain.EnvOwnedEntry{{File: ".env", Key: "DATABASE", Value: "app"}}, writes.Owned)
}
