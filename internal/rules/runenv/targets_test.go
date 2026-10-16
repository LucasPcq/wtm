package runenv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules/runenv"
)

func TestTargetsKeepTheLinksOfADeclaredEnvAndNameTheOthers(t *testing.T) {
	run := domain.RunConfig{
		EnvPorts:  []domain.EnvPortLink{{File: ".env", Key: "PORT"}, {File: "web/.env", Key: "WEB_PORT"}},
		EnvValues: []domain.EnvValueLink{{File: ".env", Key: "REALM"}, {File: "api/.env", Key: "URL"}},
	}

	kept, ignored := runenv.Targets(runenv.TargetsParams{Run: run, Files: []domain.EnvFile{{Target: ".env"}}})

	assert.Equal(t, []domain.EnvPortLink{{File: ".env", Key: "PORT"}}, kept.EnvPorts)
	assert.Equal(t, []domain.EnvValueLink{{File: ".env", Key: "REALM"}}, kept.EnvValues)
	assert.Equal(t, []runenv.Ignored{
		{Kind: runenv.LinkPort, Key: "WEB_PORT", File: "web/.env"},
		{Kind: runenv.LinkValue, Key: "URL", File: "api/.env"},
	}, ignored)
}

func TestTargetsOfARunWithNoLinkIgnoreNothing(t *testing.T) {
	run := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	kept, ignored := runenv.Targets(runenv.TargetsParams{Run: run})

	assert.Equal(t, run, kept)
	assert.Empty(t, ignored)
}
