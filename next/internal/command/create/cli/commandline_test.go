package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/service/provision"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/stretchr/testify/assert"
)

func TestTheRequestIsSpelledWithItsFlagsSortedAndQuoted(t *testing.T) {
	req := create.Request{
		Branches:    []string{"feat/a", "it's"},
		From:        "two words",
		IfNotExists: true,
		Given:       provision.Given{EnvFrom: project.EnvMain, Isolation: project.Verbatim, FastForward: new(true), Remember: []string{"env_from", "isolation"}, Ask: true},
	}

	assert.Equal(t, `wtm-next create feat/a 'it'\''s' --env-from main --ff --from 'two words' --if-not-exists --isolation verbatim --remember env_from --remember isolation --ask`, output.Line(newForm(nil).Argv("wtm-next create", req)))
}

func TestAnUnsetRequestIsTheBareCommand(t *testing.T) {
	assert.Equal(t, "wtm-next create feat/a", output.Line(newForm(nil).Argv("wtm-next create", create.Request{Branches: []string{"feat/a"}})))
	assert.Equal(t, "wtm-next create feat/a --ff=false", output.Line(newForm(nil).Argv("wtm-next create", create.Request{Branches: []string{"feat/a"}, Given: provision.Given{FastForward: new(false)}})))
}

func TestTheArgvKeepsEachWordAsTheRequestHasIt(t *testing.T) {
	req := create.Request{Branches: []string{"it's"}, From: "two words"}

	assert.Equal(t, []string{"wtm-next", "create", "it's", "--from", "two words"}, newForm(nil).Argv("wtm-next create", req))
}
