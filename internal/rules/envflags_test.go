package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidateEnvFlags(t *testing.T) {
	cases := []struct {
		name   string
		params EnvFlagsParams
		want   error
	}{
		{name: "plain", params: EnvFlagsParams{Mode: domain.EnvModeAdd}},
		{name: "json without yes", params: EnvFlagsParams{Format: domain.OutputJSON, Mode: domain.EnvModeAdd}, want: domain.ErrEnvJSONNeedsYes},
		{name: "json check", params: EnvFlagsParams{Format: domain.OutputJSON, Check: true, Mode: domain.EnvModeAdd}},
		{name: "isolation with check", params: EnvFlagsParams{Check: true, Isolation: domain.IsolationVerbatim, Mode: domain.EnvModeAdd}, want: domain.ErrEnvIsolationWithCheck},
		{name: "prune with check", params: EnvFlagsParams{Check: true, Prune: true, Mode: domain.EnvModeAdd}, want: domain.ErrEnvDecisionWithCheck},
		{name: "on-conflict with check", params: EnvFlagsParams{Check: true, OnConflictSet: true, Mode: domain.EnvModeRefresh}, want: domain.ErrEnvDecisionWithCheck},
		{name: "on-conflict in add", params: EnvFlagsParams{OnConflictSet: true, Mode: domain.EnvModeAdd}, want: domain.ErrEnvOnConflictNeedsRefresh},
		{name: "on-conflict in refresh", params: EnvFlagsParams{OnConflictSet: true, Mode: domain.EnvModeRefresh}},
	}
	for _, tc := range cases {
		if err := ValidateEnvFlags(tc.params); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}
