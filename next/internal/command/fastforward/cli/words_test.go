package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEveryFieldHasItsCommandLineWords(t *testing.T) {
	formtest.Worded[fastforward.Questions](t, words)
}
