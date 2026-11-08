package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEveryFieldHasItsCommandLineWords(t *testing.T) {
	formtest.Worded[daemonstop.Questions](t, words)
}
