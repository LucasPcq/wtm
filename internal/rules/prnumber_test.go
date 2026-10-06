package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestParsePRNumberAcceptsAPositiveInteger(t *testing.T) {
	got, err := ParsePRNumber("42")
	if err != nil || got != 42 {
		t.Fatalf("ParsePRNumber(42) = %d, %v", got, err)
	}
}

func TestParsePRNumberRefusesAnythingElseAsAUsageError(t *testing.T) {
	for _, arg := range []string{"feat/c", "0", "-3", "4.2", "#42", ""} {
		_, err := ParsePRNumber(arg)
		if err == nil {
			t.Errorf("%q accepted", arg)
			continue
		}
		if !errors.Is(err, domain.ErrUsage) {
			t.Errorf("%q: %v is not a usage error", arg, err)
		}
	}
}
