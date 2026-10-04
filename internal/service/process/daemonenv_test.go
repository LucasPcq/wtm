package process

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A daemon outlives the command that started it, so a correlation id that
// command carried would otherwise stamp every job's wtm calls as the caller's.
func TestTheDaemonDoesNotInheritTheCallersCorrelationID(t *testing.T) {
	got := daemonEnv([]string{"PATH=/bin", domain.EnvCorrelationID + "=popup-1", "HOME=/h"})

	if !slices.Equal(got, []string{"PATH=/bin", "HOME=/h"}) {
		t.Fatalf("env = %v", got)
	}
}
