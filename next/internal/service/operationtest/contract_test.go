package operationtest

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestAUnitOrAPhaseStartedAndNotFinishedAfterIsUnfinished(t *testing.T) {
	events := []domain.Event{
		domain.UnitFinished{Subject: "early"},
		domain.UnitStarted{Subject: "early"},
		domain.UnitStarted{Subject: "a"},
		domain.PhaseStarted{Subject: "a", Phase: "hooks"},
		domain.PhaseFinished{Subject: "a", Phase: "hooks"},
		domain.PhaseStarted{Subject: "a", Phase: "env"},
		domain.UnitFinished{Subject: "a"},
	}

	assert.Equal(t, []string{"unit early", "phase env of a"}, unfinished(events))
}
