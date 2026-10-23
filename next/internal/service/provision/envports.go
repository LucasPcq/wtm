package provision

import (
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

// codePortsLeftAlone warns that provisioning left ports of a worktree's .env files alone, its params counting them and its ports saying which and why.
const codePortsLeftAlone = "provision.env_ports_left_alone"

// portsLeftAlone warns of the ports plan could not shift, counting and carrying them; none when there are none.
func portsLeftAlone(plan env.Redacted) []domain.Warning {
	anomalies := env.Plan(plan).Anomalies()
	if len(anomalies) == 0 {
		return nil
	}

	return []domain.Warning{{Code: codePortsLeftAlone, Message: "Env ports left alone", Params: domain.Params{"count": strconv.Itoa(len(anomalies))}, Ports: anomalies}}
}
