package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEveryFieldHasItsCommandLineWords(t *testing.T) {
	formtest.Worded[envshow.Questions](t, words)
}
