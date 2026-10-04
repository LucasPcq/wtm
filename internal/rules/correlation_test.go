package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidateCorrelationID(t *testing.T) {
	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{name: "empty", value: "", ok: true},
		{name: "plain", value: "herdr:popup:42", ok: true},
		{name: "at the limit", value: strings.Repeat("a", domain.CorrelationIDMaxBytes), ok: true},
		{name: "over the limit", value: strings.Repeat("a", domain.CorrelationIDMaxBytes+1)},
		{name: "newline", value: "a\nb"},
		{name: "nul", value: "a\x00b"},
		{name: "del", value: "a\x7fb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCorrelationID(tc.value)
			if tc.ok && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !tc.ok && !errors.Is(err, domain.ErrInvalidCorrelationID) {
				t.Fatalf("err = %v, want ErrInvalidCorrelationID", err)
			}
		})
	}
}
