package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestUsageKeepsTheMessageAndTheCause(t *testing.T) {
	cause := errors.New("--all cannot be combined with branch arguments")
	err := Usage(cause)
	if err.Error() != cause.Error() {
		t.Errorf("message = %q, want %q", err.Error(), cause.Error())
	}
	if !errors.Is(err, domain.ErrUsage) || !errors.Is(err, cause) {
		t.Errorf("%v does not carry both ErrUsage and its cause", err)
	}
	if Usage(nil) != nil {
		t.Error("Usage(nil) is not nil")
	}
}
