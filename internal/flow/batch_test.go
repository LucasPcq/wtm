package flow

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestBatchErrorMarksOnlyABatchAsReported(t *testing.T) {
	cause := errors.New("hook failed")
	if got := BatchError(BatchErrorParams{First: cause}); got != cause {
		t.Errorf("single = %v, want the cause as it is", got)
	}
	if got := BatchError(BatchErrorParams{First: cause, Batch: true}); !errors.Is(got, domain.ErrAborted) || !errors.Is(got, cause) {
		t.Errorf("batch = %v, want ErrAborted wrapping the cause", got)
	}
	if got := BatchError(BatchErrorParams{Batch: true}); got != nil {
		t.Errorf("success = %v, want nil", got)
	}
}
