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

type MainAddressingParams struct {
	Names domain.EnvPortPlan
	Ports domain.EnvPortPlan
}

// MainAddressing reads the main checkout's .env from the plans each mode would
// apply: current is the mode its linked values spell, and matters is false when
// both modes write the same values, so there is nothing to choose.
func MainAddressing(params MainAddressingParams) (current domain.Addressing, matters bool) {
	current = domain.AddressingNames
	if AddressedByPort(params.Names) {
		current = domain.AddressingPorts
	}
	ports := make(map[string]string, len(params.Ports.Entries))
	for _, entry := range params.Ports.Entries {
		ports[entry.File+"\x00"+entry.Key] = settledValue(entry)
	}
	for _, entry := range params.Names.Entries {
		if value, ok := ports[entry.File+"\x00"+entry.Key]; ok && value != settledValue(entry) {
			return current, true
		}
	}
	return current, false
}

func settledValue(entry domain.EnvPortEntry) string {
	if entry.Status == domain.EnvPortStatusRewrite {
		return entry.NewValue
	}
	return entry.CurrentValue
}

type EnvAddressingParams struct {
	Requested domain.Addressing
	Project   domain.Addressing
	IsMain    bool
}

// ValidateEnvAddressing refuses an --addressing a worktree cannot take: names
// on a project addressed by ports, which publishes none, and anything but the
// project's mode on a linked worktree, which always follows run.toml.
func ValidateEnvAddressing(params EnvAddressingParams) error {
	switch {
	case params.Requested == "" || params.Requested == params.Project:
		return nil
	case params.Requested == domain.AddressingNames:
		return domain.ErrEnvAddressingProjectPorts
	case !params.IsMain:
		return domain.ErrEnvAddressingMainOnly
	}
	return nil
}
