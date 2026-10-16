package runenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestTargetsKeepTheLinksOfADeclaredEnvAndNameTheOthers(t *testing.T) {
	config := run.Config{
		EnvPorts:  []run.EnvPortLink{{File: ".env", Key: "PORT"}, {File: "web/.env", Key: "WEB_PORT"}},
		EnvValues: []run.EnvValueLink{{File: ".env", Key: "REALM"}, {File: "api/.env", Key: "URL"}},
	}

	kept, ignored := Targets(TargetsParams{Run: config, Files: []project.EnvFile{{Target: ".env"}}})

	assert.Equal(t, []run.EnvPortLink{{File: ".env", Key: "PORT"}}, kept.EnvPorts)
	assert.Equal(t, []run.EnvValueLink{{File: ".env", Key: "REALM"}}, kept.EnvValues)
	assert.Equal(t, []Ignored{
		{Kind: LinkPort, Key: "WEB_PORT", File: "web/.env"},
		{Kind: LinkValue, Key: "URL", File: "api/.env"},
	}, ignored)
}

func TestTargetsOfARunWithNoLinkIgnoreNothing(t *testing.T) {
	config := run.Config{Jobs: []run.Job{{Name: "web"}}}

	kept, ignored := Targets(TargetsParams{Run: config})

	assert.Equal(t, config, kept)
	assert.Empty(t, ignored)
}
