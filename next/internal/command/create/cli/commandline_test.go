package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestTheRequestIsSpelledWithItsFlagsSortedAndQuoted(t *testing.T) {
	req := create.Request{
		Branches:    []string{"feat/a", "it's"},
		From:        "two words",
		EnvFrom:     project.EnvMain,
		Isolation:   project.Verbatim,
		FastForward: new(true),
		IfNotExists: true,
		Remember:    []string{"env_from", "isolation"},
		Ask:         true,
	}

	assert.Equal(t, `wtm-next create feat/a 'it'\''s' --env-from main --ff --from 'two words' --if-not-exists --isolation verbatim --remember env_from --remember isolation --ask`, createLine{Command: "wtm-next create", Request: req}.String())
}

func TestAnUnsetRequestIsTheBareCommand(t *testing.T) {
	assert.Equal(t, "wtm-next create feat/a", createLine{Command: "wtm-next create", Request: create.Request{Branches: []string{"feat/a"}}}.String())
	assert.Equal(t, "wtm-next create feat/a --ff=false", createLine{Command: "wtm-next create", Request: create.Request{Branches: []string{"feat/a"}, FastForward: new(false)}}.String())
}
