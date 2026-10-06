package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJSONNeedsYesNamesEveryWayOut(t *testing.T) {
	cases := map[string][]string{
		"--output json requires --yes (prompts cannot run in JSON mode)":              {domain.FlagYes},
		"--output json requires --yes or --dry-run (prompts cannot run in JSON mode)": {domain.FlagYes, domain.FlagDryRun},
	}
	for want, flags := range cases {
		err := JSONNeedsYes(flags)
		if err.Error() != want {
			t.Errorf("JSONNeedsYes(%v) = %q, want %q", flags, err.Error(), want)
		}
		if !errors.Is(err, domain.ErrUsage) {
			t.Errorf("JSONNeedsYes(%v) is not a usage error", flags)
		}
	}
}
