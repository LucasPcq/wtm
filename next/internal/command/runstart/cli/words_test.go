package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEveryFieldHasItsCommandLineWords(t *testing.T) {
	formtest.Worded[runstart.Questions](t, words)
}
