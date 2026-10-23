package provision

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/stretchr/testify/assert"
)

func TestThePortsLeftAloneAreOneWarningCountingAndCarryingThem(t *testing.T) {
	plan := env.Redacted{Entries: []env.Entry{
		{File: ".env", Key: "DB", Port: "pg", Status: env.StatusMissingKey},
		{File: ".env", Key: "PORT", Status: env.StatusRewrite},
	}}

	assert.Equal(t, []domain.Warning{{Code: codePortsLeftAlone, Message: "Env ports left alone", Params: domain.Params{"count": "1"}, Ports: plan.Entries[:1]}}, portsLeftAlone(plan))
	assert.Empty(t, portsLeftAlone(env.Redacted{}))
}
