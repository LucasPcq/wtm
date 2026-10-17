package process

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A test reaching the real spawner used to fork its own binary as "daemon":
// that binary reran the suite, which forked again, in a chain nothing reaped.
func TestATestBinaryNeverForksItselfAsTheDaemon(t *testing.T) {
	if err := StartDaemon(DaemonParams{}); !errors.Is(err, domain.ErrDaemonForkInTest) {
		t.Fatalf("err = %v, want ErrDaemonForkInTest: the test binary must never be forked", err)
	}
}
