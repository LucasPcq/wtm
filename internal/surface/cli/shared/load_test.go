package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

func TestAReadCutShortByAnInterruptIsNoAnswer(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	err := Load(cmd, components.LoadingParams{Work: func() error {
		cancel()
		return nil
	}})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want the interrupt rather than a listing it cut short", err)
	}
}
