package components

import (
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// errorBanner renders an inline error line "  ✗ Message" matching output.Error.
// The first rune of msg is capitalized so wizard errors read like sentences.
func errorBanner(msg string) string {
	var b strings.Builder
	b.WriteString(styles.Indent)
	b.WriteString(styles.DangerText.Render(domain.GlyphFailure))
	b.WriteString(" ")
	b.WriteString(styles.DangerText.Render(capitalizeFirst(msg)))
	return b.String()
}

// warningBanner renders an inline warning line "  ! Message" matching
// output.Warning. Capitalizes the first rune for readability.
func warningBanner(msg string) string {
	var b strings.Builder
	b.WriteString(styles.Indent)
	b.WriteString(styles.BadgeWarning.Render(domain.GlyphAttention))
	b.WriteString(" ")
	b.WriteString(styles.Warning.Render(capitalizeFirst(msg)))
	return b.String()
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
