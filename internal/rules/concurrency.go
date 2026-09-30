package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ConcurrencyParams struct {
	// Exclusive and Parallel are the flags, which override the config for one run.
	Exclusive bool
	Parallel  bool
	Config    domain.Concurrency
	// OthersRunning says another worktree has jobs up. Without one there is
	// nothing to stop, so there is nothing to decide either.
	OthersRunning bool
	// Selection is how many worktrees this run starts. More than one contradicts
	// a settled `exclusive`, which means one stack at a time: bringing up three
	// is exactly what that answer refused, whether or not anything else is up.
	Selection int
	// Clashes says a port this run needs is bound in another worktree. Running
	// side by side is then not an answer at all, whatever the flags or the
	// config prefer.
	Clashes bool
}

// ConcurrencyDecision is what a run does about the other worktrees, and whether
// anybody still has to be asked. Value is always usable: Ask only says that a
// fully interactive run may offer the choice, never that it must.
type ConcurrencyDecision struct {
	Value domain.Concurrency
	Ask   bool
	// Contradiction says the question is about a run that contradicts the
	// project's settled answer, rather than about a project that has never given
	// one. They are two different questions and have to read as such.
	Contradiction bool
	// Clash says the only ways out are stopping the other worktrees or not
	// starting: nobody may be left to find out through a bind error.
	Clash bool
}

// DecideConcurrency resolves the flags, then the config, then falls back to
// leaving the other worktrees alone. Parallel is the safe default in the sense
// the bypass model means: it is the answer that stops nothing.
func DecideConcurrency(params ConcurrencyParams) ConcurrencyDecision {
	if params.Clashes {
		if params.Exclusive || (params.Config == domain.ConcurrencyExclusive && !params.Parallel) {
			return ConcurrencyDecision{Value: domain.ConcurrencyExclusive}
		}
		return ConcurrencyDecision{Value: domain.ConcurrencyExclusive, Ask: true, Clash: true}
	}
	// The contradiction outranks everything the config settled: the answer it
	// holds cannot be applied to this run, so it has to be put to whoever can
	// answer — and stand aside where nobody can.
	if contradictsSettledExclusive(params) {
		return ConcurrencyDecision{Value: domain.ConcurrencyParallel, Ask: true, Contradiction: true}
	}
	if !params.OthersRunning {
		return ConcurrencyDecision{Value: domain.ConcurrencyParallel}
	}
	if params.Exclusive {
		return ConcurrencyDecision{Value: domain.ConcurrencyExclusive}
	}
	if params.Parallel {
		return ConcurrencyDecision{Value: domain.ConcurrencyParallel}
	}
	if params.Config == domain.ConcurrencyExclusive || params.Config == domain.ConcurrencyParallel {
		return ConcurrencyDecision{Value: params.Config}
	}
	return ConcurrencyDecision{Value: domain.ConcurrencyParallel, Ask: true}
}

// contradictsSettledExclusive reports a run that brings up several worktrees
// against a project that settled on one at a time. A flag settles the run
// either way, so neither of them leaves a contradiction to resolve.
func contradictsSettledExclusive(params ConcurrencyParams) bool {
	return params.Selection > 1 &&
		params.Config == domain.ConcurrencyExclusive &&
		!params.Exclusive && !params.Parallel
}

// ValidateConcurrency refuses a value that is neither of the two. A typo would
// otherwise read as "not answered yet" and bring the question back for good.
func ValidateConcurrency(cfg domain.RunConfig) []string {
	switch cfg.Concurrency {
	case "", domain.ConcurrencyParallel, domain.ConcurrencyExclusive:
		return nil
	default:
		return []string{fmt.Sprintf("unknown concurrency %q (expected %q or %q)",
			cfg.Concurrency, domain.ConcurrencyParallel, domain.ConcurrencyExclusive)}
	}
}
