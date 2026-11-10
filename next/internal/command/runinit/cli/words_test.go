package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEveryFieldOfRunInitIsWorded(t *testing.T) {
	formtest.Worded(t, words)
}
