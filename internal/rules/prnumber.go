package rules

import (
	"fmt"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

func ParsePRNumber(arg string) (int, error) {
	number, err := strconv.Atoi(arg)
	if err != nil || number <= 0 {
		return 0, flagValueError{text: fmt.Sprintf(domain.CheckoutPRNumberInvalidFmt, arg)}
	}
	return number, nil
}
