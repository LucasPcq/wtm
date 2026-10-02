package flow

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type Progress struct {
	Branch   string
	Position int
	Total    int
}

type BatchErrorParams struct {
	First error
	Batch bool
}

// BatchError keeps a single item failing exactly as it always did, and marks a
// batch's failure as already reported: its readout named every one.
func BatchError(params BatchErrorParams) error {
	if params.First == nil || !params.Batch {
		return params.First
	}
	return fmt.Errorf("%w: %w", domain.ErrAborted, params.First)
}
