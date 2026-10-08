package flow

import (
	"context"
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Interrupted reads a failure an interrupt caused as the interrupt — a hook
// stopped by its signal, a git that found its context done — so the item's
// exit code says cancelled rather than failed.
func Interrupted(ctx context.Context, err error) error {
	if err == nil || ctx.Err() == nil || errors.Is(err, domain.ErrCancelled) {
		return err
	}
	return fmt.Errorf("%w: %w", domain.ErrCancelled, err)
}
