package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEveryFieldHasItsCommandLineWords(t *testing.T) {
	formtest.Worded[reparent.Questions](t, words)
}
