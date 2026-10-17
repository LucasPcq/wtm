package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/stretchr/testify/assert"
)

func TestTheRequestIsSpelledWithItsFlagsSortedAndQuoted(t *testing.T) {
	req := extract.Request{
		Source:      "feat/login",
		Files:       []string{"apps/api", "two words.txt"},
		To:          "feat/split",
		From:        "main",
		FastForward: new(true),
		Isolation:   project.Verbatim,
		Keep:        new(false),
		OnConflict:  extract.OnConflictResolve,
		Remember:    []string{"isolation"},
		Ask:         true,
	}

	assert.Equal(t, `wtm-next extract feat/login --ff --files apps/api --files 'two words.txt' --from main --isolation verbatim --keep=false --on-conflict resolve --remember isolation --to feat/split --ask`, output.Line(newForm(nil).Argv("wtm-next extract", req)))
}

func TestAnUnsetRequestIsTheBareCommand(t *testing.T) {
	assert.Equal(t, "wtm-next extract", output.Line(newForm(nil).Argv("wtm-next extract", extract.Request{})))
}

func TestTheArgvKeepsEachWordAsTheRequestHasIt(t *testing.T) {
	req := extract.Request{Source: "it's", Files: []string{"two words.txt"}}

	assert.Equal(t, []string{"wtm-next", "extract", "it's", "--files", "two words.txt"}, newForm(nil).Argv("wtm-next extract", req))
}
