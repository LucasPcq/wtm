package rules

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
)

func ValidateCorrelationID(value string) error {
	if len(value) > domain.CorrelationIDMaxBytes {
		return fmt.Errorf(domain.CorrelationIDInvalidFmt, domain.EnvCorrelationID, domain.CorrelationIDTooLong, domain.ErrInvalidCorrelationID)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf(domain.CorrelationIDInvalidFmt, domain.EnvCorrelationID, domain.CorrelationIDControl, domain.ErrInvalidCorrelationID)
	}
	return nil
}
