package form

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

const followUpTitle = "Run it now?"

type FollowUpParams struct {
	Command string
	Note    string
}

// FollowUp reads backing out as a no.
func FollowUp(params FollowUpParams) (bool, error) {
	description := params.Command
	if params.Note != "" {
		description += "\n" + params.Note
	}
	confirmed, err := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{Title: followUpTitle, Description: description}))
	if errors.Is(err, domain.ErrUserAborted) {
		return false, nil
	}
	return confirmed, err
}
