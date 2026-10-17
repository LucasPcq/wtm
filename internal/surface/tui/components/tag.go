package components

import "github.com/LucasPcq/wtm/internal/domain"

// TagVariantOf maps a declared tone onto this package's palette. It lives here
// so the two surfaces that render tagged options cannot drift apart on what
// "danger" looks like.
func TagVariantOf(tone domain.Tone) TagVariant {
	switch tone {
	case domain.ToneSuccess:
		return TagSuccess
	case domain.ToneWarning:
		return TagWarning
	case domain.ToneDanger:
		return TagDanger
	default:
		return TagNeutral
	}
}

// BadgeVariantOf is TagVariantOf for the trailing badges a select row carries,
// and lives beside it for the same reason: two surfaces render them.
func BadgeVariantOf(tone domain.Tone) BadgeVariant {
	switch tone {
	case domain.ToneSuccess:
		return BadgeSuccess
	case domain.ToneWarning:
		return BadgeWarning
	case domain.ToneDanger:
		return BadgeDanger
	default:
		return BadgeNeutral
	}
}
