package runenv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules/runenv"
)

func TestOnlyALinkedWorktreesLinksNeedItsNumber(t *testing.T) {
	ports := domain.RunConfig{EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "PORT"}}}
	values := domain.RunConfig{EnvValues: []domain.EnvValueLink{{File: ".env", Key: "REALM"}}}

	assert.True(t, runenv.PortsNeedOrdinal(runenv.PortsNeedOrdinalParams{Case: runenv.Linked{}, Run: ports}))
	assert.True(t, runenv.PortsNeedOrdinal(runenv.PortsNeedOrdinalParams{Case: runenv.Linked{}, Run: values}))
	assert.False(t, runenv.PortsNeedOrdinal(runenv.PortsNeedOrdinalParams{Case: runenv.Linked{}, Run: domain.RunConfig{}}))
	assert.False(t, runenv.PortsNeedOrdinal(runenv.PortsNeedOrdinalParams{Case: runenv.Verbatim{}, Run: ports}))
	assert.False(t, runenv.PortsNeedOrdinal(runenv.PortsNeedOrdinalParams{Case: runenv.Main{}, Run: ports}))
}
