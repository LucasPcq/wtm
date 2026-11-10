package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

func TestEverySettingIsWorded(t *testing.T) {
	formtest.Worded[initialize.Questions](t, words)
}
