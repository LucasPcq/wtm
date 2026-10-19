package provision

import (
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

// portsLeftAlone warns of the ports plan could not shift, counting them; none when there are none.
func portsLeftAlone(plan env.Redacted) []domain.Warning {
	count := len(env.Plan(plan).Anomalies())
	if count == 0 {
		return nil
	}

	return []domain.Warning{{Code: env.CodePortsLeftAlone, Message: "Env ports left alone", Params: domain.Params{"count": strconv.Itoa(count)}}}
}
