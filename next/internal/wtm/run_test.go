package wtm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAJobRunsComposeInEitherSpellingOfItsStartOrStop(t *testing.T) {
	assert.True(t, JobConfig{Cmd: "docker compose up -d"}.RunsCompose())
	assert.True(t, JobConfig{Cmd: "make up", Stop: "docker-compose down"}.RunsCompose())
	assert.False(t, JobConfig{Cmd: "pnpm dev"}.RunsCompose())
}

func TestAJobHasANamespaceOnlyWhenItNamesAndCreatesOne(t *testing.T) {
	assert.True(t, JobConfig{Namespace: &JobNamespaceConfig{Name: "{worktree}", Create: "createdb {namespace}"}}.HasNamespace())
	assert.False(t, JobConfig{Namespace: &JobNamespaceConfig{Name: "{worktree}"}}.HasNamespace())
	assert.False(t, JobConfig{}.HasNamespace())
}
