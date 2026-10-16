package runenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestOnlyALinkedWorktreesLinksNeedItsNumber(t *testing.T) {
	ports := run.Config{EnvPorts: []run.EnvPortLink{{File: ".env", Key: "PORT"}}}
	values := run.Config{EnvValues: []run.EnvValueLink{{File: ".env", Key: "REALM"}}}

	assert.True(t, PortsNeedOrdinal(PortsNeedOrdinalParams{Case: Linked{}, Run: ports}))
	assert.True(t, PortsNeedOrdinal(PortsNeedOrdinalParams{Case: Linked{}, Run: values}))
	assert.False(t, PortsNeedOrdinal(PortsNeedOrdinalParams{Case: Linked{}, Run: run.Config{}}))
	assert.False(t, PortsNeedOrdinal(PortsNeedOrdinalParams{Case: Verbatim{}, Run: ports}))
}

func TestHooksReadTheRunEnvOnlyOfAWorktreeThatChoseAndAComposeStack(t *testing.T) {
	compose := run.Config{Jobs: []run.Job{{Name: "db", Cmd: "docker compose up -d"}}}
	script := run.Config{Jobs: []run.Job{{Name: "web", Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}}}
	cases := []struct {
		name   string
		params HooksReadRunParams
		want   bool
	}{
		{"isolated with a compose job", HooksReadRunParams{Run: compose, Isolation: run.Isolated}, true},
		{"verbatim with a compose job", HooksReadRunParams{Run: compose, Isolation: run.Verbatim}, true},
		{"never chose", HooksReadRunParams{Run: compose}, false},
		{"no compose job", HooksReadRunParams{Run: script, Isolation: run.Isolated}, false},
		{"no run config", HooksReadRunParams{Isolation: run.Isolated}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HooksReadRun(tc.params))
		})
	}
}
