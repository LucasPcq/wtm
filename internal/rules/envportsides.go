package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvPortMovesTo renders where a value's ports land. It is the one renderer a
// report has for a port link, and it reads the plan only — numbers and the
// address wtm builds — never the value, so no shape of value can leak through
// it.
func EnvPortMovesTo(moves []domain.EnvPortMove) string {
	sides := make([]string, 0, len(moves))
	for _, move := range moves {
		if move.Origin != "" {
			sides = append(sides, move.Origin)
			continue
		}
		sides = append(sides, envPortSide(move.Resolved))
	}
	return strings.Join(sides, domain.EnvOriginJoin)
}

// EnvBasePorts renders the declared ports a restored value goes back to.
func EnvBasePorts(ports []int) string {
	sides := make([]string, 0, len(ports))
	for _, port := range ports {
		sides = append(sides, envPortSide(port))
	}
	return strings.Join(sides, domain.EnvOriginJoin)
}

func envPortSide(port int) string {
	return fmt.Sprintf(domain.EnvPortSideFmt, port)
}
