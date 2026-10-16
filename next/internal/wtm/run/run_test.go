package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAJobRunsComposeInEitherSpellingOfItsStartOrStop(t *testing.T) {
	assert.True(t, Job{Cmd: "docker compose up -d"}.RunsCompose())
	assert.True(t, Job{Cmd: "make up", Stop: "docker-compose down"}.RunsCompose())
	assert.False(t, Job{Cmd: "pnpm dev"}.RunsCompose())
}

func TestAJobHasANamespaceOnlyWhenItNamesAndCreatesOne(t *testing.T) {
	assert.True(t, Job{Namespace: &Namespace{Name: "{worktree}", Create: "createdb {namespace}"}}.HasNamespace())
	assert.False(t, Job{Namespace: &Namespace{Name: "{worktree}"}}.HasNamespace())
	assert.False(t, Job{}.HasNamespace())
}
