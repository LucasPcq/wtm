package cli

import (
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheRequestIsSpelledWithItsFlagsSortedAndQuoted(t *testing.T) {
	req := create.Request{
		Branches:    []string{"feat/a", "it's"},
		From:        "two words",
		EnvFrom:     project.EnvMain,
		Isolation:   run.Verbatim,
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

func TestAFollowUpIsTheCommandItNamesWithItsBranchesFirst(t *testing.T) {
	line, err := followUpLine(&wtm.FollowUp{Command: "clean", Request: json.RawMessage(`{"branches":["feat/x"],"force":true,"keep_branch":false,"note":""}`)})

	require.NoError(t, err)
	assert.Equal(t, "wtm clean feat/x --force", line)
}

func TestAFollowUpWhoseRequestIsNoObjectIsAnError(t *testing.T) {
	_, err := followUpLine(&wtm.FollowUp{Command: "clean", Request: json.RawMessage(`["feat/x"]`)})

	assert.Error(t, err)
}
