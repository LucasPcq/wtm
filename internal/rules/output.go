package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsHumanFormat reports whether the output format targets a human reader and
// therefore must be framed with the canonical top/bottom blank lines. JSON is
// the only machine format and is always emitted flush, without decorative
// padding.
func IsHumanFormat(format string) bool {
	return format != domain.OutputJSON
}

// OutputFormats is every --output value a command accepts: text and json,
// then the extra ones it declares in domain.AnnotationOutputFormats.
func OutputFormats(extra string) []string {
	formats := []string{domain.OutputText, domain.OutputJSON}
	for _, format := range strings.Split(extra, ",") {
		if format = strings.TrimSpace(format); format != "" {
			formats = append(formats, format)
		}
	}
	return formats
}
