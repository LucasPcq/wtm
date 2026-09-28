package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// PendingOriginRewrites counts the keys a `wtm env` on this worktree would move
// from a port to a named origin. It is what tells a published name apart from a
// working one: the route exists as soon as the job runs, but the app behind it
// only answers on that origin once its .env says so.
func PendingOriginRewrites(plan domain.EnvPortPlan) int {
	pending := 0
	for _, entry := range EnvPortRewrites(plan) {
		if entry.Addressing == domain.AddressingNames {
			pending++
		}
	}
	return pending
}

// AddressedByPort reports that this worktree's linked values still spell the
// jobs' addresses as loopback ports. The published name is then the broken
// entrance and the port is the working one — both sides of a cross-origin call
// agree on the port — so a surface hands out the ports until `wtm env` settles
// the file.
//
// A value already carrying a named origin is not that, even a stale one: the
// .env has moved to names, and only its port has to catch up.
func AddressedByPort(plan domain.EnvPortPlan) bool {
	for _, entry := range EnvPortRewrites(plan) {
		if entry.Addressing == domain.AddressingNames && hasLoopbackOrigin(entry.CurrentValue) {
			return true
		}
	}
	return false
}

func hasLoopbackOrigin(value string) bool {
	for _, element := range splitList(value) {
		_, authority, ok := splitOrigin(element.value)
		if !ok {
			continue
		}
		if host, _ := splitHostPort(authority); isLoopbackHost(host) {
			return true
		}
	}
	return false
}

type AddressingDriftParams struct {
	Worktree string
	Plan     domain.EnvPortPlan
}

// AddressingDriftLine is the single line a surface showing this worktree's URLs
// puts under them, empty when there is nothing to say. One line whatever the
// number of keys: the reader needs the worktree and the command, not a tally.
func AddressingDriftLine(params AddressingDriftParams) string {
	if params.Worktree == "" || PendingOriginRewrites(params.Plan) == 0 {
		return ""
	}
	// Two states, two sentences: a .env still on ports is being served its
	// ports, which is not a fault to report but a choice to explain; a .env on
	// names whose origins went stale is served them and answers none.
	if AddressedByPort(params.Plan) {
		return fmt.Sprintf(domain.AddressingPortedFmt, params.Worktree, params.Worktree)
	}
	return fmt.Sprintf(domain.AddressingDriftFmt, params.Worktree, params.Worktree)
}

// AddressingDriftLines is the same warning over as many worktrees as a run
// covers, one line each. Nil when every worktree given is aligned.
func AddressingDriftLines(drifts []AddressingDriftParams) []string {
	var lines []string
	for _, drift := range drifts {
		if line := AddressingDriftLine(drift); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
