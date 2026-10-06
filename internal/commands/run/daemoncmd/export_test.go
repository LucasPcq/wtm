package daemoncmd

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/tui/components"
)

func StubConfirm(t *testing.T, answer func(components.ConfirmModel) (bool, error)) {
	t.Helper()
	previous := runConfirm
	runConfirm = answer
	t.Cleanup(func() { runConfirm = previous })
}
