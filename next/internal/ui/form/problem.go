package form

import (
	"errors"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// Problem is a field problem as a step shows it, its path spelled as label: the first letter of its template's own words is capitalised, never that of a value it starts with (feat/other already has a worktree).
func Problem(found domain.FieldError, label string) error {
	message := found.Message(label)
	if strings.HasPrefix(found.Template, "{") {
		return errors.New(message)
	}

	return errors.New(capitalized(message))
}

// Sentence is a check's error as a step shows it: a Go error, lowercase by convention, starting with a capital.
func Sentence(err error) error {
	if err == nil {
		return nil
	}

	return errors.New(capitalized(err.Error()))
}

func capitalized(text string) string {
	if text == "" {
		return text
	}
	runes := []rune(text)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
