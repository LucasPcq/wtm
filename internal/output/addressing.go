package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type AddressingResult struct {
	Addressing domain.Addressing `json:"addressing"`
	Previous   domain.Addressing `json:"previous"`
	Changed    bool              `json:"changed"`
	Settled    []string          `json:"settled"`
	Pending    []string          `json:"pending"`
	MainLeft   string            `json:"main_left,omitempty"`
}

// AddressingSwitched is the counted readout of `run addressing`: the mode, then
// how many worktrees moved with it and how many were left behind. Which ones is
// in the JSON and in `wtm env`, the command whose subject they are.
func AddressingSwitched(w io.Writer, result AddressingResult) {
	if result.Changed {
		Success(w, fmt.Sprintf(domain.AddressingSwitchedFmt, result.Previous, result.Addressing))
	} else {
		Unchanged(w, fmt.Sprintf(domain.AddressingUnchangedFmt, result.Addressing))
	}
	if len(result.Settled) > 0 {
		Success(w, fmt.Sprintf(domain.AddressingSettledFmt, rules.WorktreeCountLabel(len(result.Settled))))
	}
	if len(result.Pending) > 0 {
		Warning(w, fmt.Sprintf(domain.AddressingPendingFmt, rules.WorktreeCountLabel(len(result.Pending))))
	}
	if result.MainLeft != "" {
		Warning(w, fmt.Sprintf(domain.AddressingMainLeftFmt, result.MainLeft, result.MainLeft))
	}
}

func WriteAddressingResultJSON(w io.Writer, result AddressingResult) error {
	if result.Settled == nil {
		result.Settled = []string{}
	}
	if result.Pending == nil {
		result.Pending = []string{}
	}
	return encodeJSON(w, result)
}
