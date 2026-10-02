package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestInvalidFlagValueIsAUsageError(t *testing.T) {
	err := InvalidFlagValue(InvalidFlagValueParams{Flag: "from", Value: "x", Allowed: []string{"example", "main", "parent"}})
	if got := err.Error(); got != `invalid --from value "x": use example, main or parent` {
		t.Errorf("message = %q", got)
	}
	if !errors.Is(err, domain.ErrUsage) || ExitCode(err) != domain.ExitCodeUsage {
		t.Errorf("err = %v, want it to exit %d", err, domain.ExitCodeUsage)
	}
}
