package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/configshow"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEveryFieldHasItsCommandLineWords(t *testing.T) {
	formtest.Worded[configshow.Questions](t, words)
}
