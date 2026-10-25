package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEveryFieldHasItsCommandLineWords(t *testing.T) {
	formtest.Worded[list.Questions](t, words)
}
