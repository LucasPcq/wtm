package form

import (
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

func (s *session) badgesOf(badges []kernel.Badge) []components.Badge {
	shown := make([]components.Badge, 0, len(badges))
	for _, badge := range badges {
		shown = append(shown, s.badgeOf(badge))
	}
	return shown
}

func (s *session) badgeOf(badge kernel.Badge) components.Badge {
	return components.Badge{Text: s.message(badge.Code, badge.Params), Variant: components.BadgeNeutral}
}
