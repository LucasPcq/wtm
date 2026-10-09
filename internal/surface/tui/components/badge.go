// Package components provides reusable Bubbletea UI primitives.
package components

import "github.com/LucasPcq/wtm/internal/styles"

// BadgeVariant determines the color scheme of a badge chip.
type BadgeVariant int

const (
	// BadgeNeutral renders gray text / a gray chip.
	BadgeNeutral BadgeVariant = iota
	// BadgeAccent renders accent-colored text (e.g. the "parent" tag).
	BadgeAccent
	// BadgeSuccess renders green text / a green chip.
	BadgeSuccess
	// BadgeWarning renders orange text / an orange chip.
	BadgeWarning
	// BadgeDanger renders red text / a red chip.
	BadgeDanger
)

// Badge is a small status indicator. It renders either as lightweight colored
// text (a left-column tag, via Render) or as a filled chip with an optional
// leading glyph (a right-aligned status pill, via RenderPill).
type Badge struct {
	Text    string
	Variant BadgeVariant
	// Glyph is an optional leading symbol for pills (e.g. "●", "✓", "⚠").
	Glyph string
}

// Render returns the badge as lightweight colored text, used for left-column
// tags so rows stay compact. The filled chip styles (styles.Badge*) are reserved
// for RenderPill and the "!" markers in output blocks and banners.
func (b Badge) Render() string {
	switch b.Variant {
	case BadgeAccent:
		return styles.Primary.Render(b.Text)
	case BadgeSuccess:
		return styles.Success.Render(b.Text)
	case BadgeWarning:
		return styles.Warning.Render(b.Text)
	case BadgeDanger:
		return styles.DangerText.Render(b.Text)
	default:
		return styles.Muted.Render(b.Text)
	}
}

// RenderPill returns the badge as a filled chip with its optional leading glyph,
// used for the right-aligned status indicator so it scans at a glance.
func (b Badge) RenderPill() string {
	label := b.Text
	if b.Glyph != "" {
		label = b.Glyph + " " + b.Text
	}
	switch b.Variant {
	case BadgeSuccess:
		return styles.BadgeSuccess.Render(label)
	case BadgeWarning:
		return styles.BadgeWarning.Render(label)
	case BadgeDanger:
		return styles.BadgeDanger.Render(label)
	default:
		return styles.Badge.Render(label)
	}
}
