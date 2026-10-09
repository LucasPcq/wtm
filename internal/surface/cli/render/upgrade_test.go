package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

func TestUpgradeResultJSON(t *testing.T) {
	var buf bytes.Buffer
	err := output.UpgradeResultJSON(&buf, domain.UpgradeResult{
		Installed: "0.26.1",
		Latest:    "0.27.0",
		UpToDate:  false,
		Method:    domain.InstallStandalone,
		Action:    domain.UpgradeActionReplaced,
	})
	if err != nil {
		t.Fatalf("UpgradeResultJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := map[string]any{
		"installed":  "0.26.1",
		"latest":     "0.27.0",
		"up_to_date": false,
		"method":     "standalone",
		"action":     "replaced",
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("json[%q] = %v, want %v", key, got[key], value)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("json has %d keys, want %d: %v", len(got), len(want), got)
	}
}

func TestUpdateNoticeNamesTheCommand(t *testing.T) {
	cases := []struct {
		name   string
		method domain.InstallMethod
		want   string
	}{
		{"standalone points at wtm upgrade", domain.InstallStandalone, "wtm upgrade"},
		{"homebrew points at brew", domain.InstallHomebrew, "brew upgrade LucasPcq/tap/wtm"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			output.UpdateNotice(&buf, output.UpdateNoticeParams{
				Current: "0.26.1",
				Latest:  "0.27.0",
				Method:  tc.method,
			})

			got := buf.String()
			for _, fragment := range []string{"0.26.1", "0.27.0", tc.want} {
				if !strings.Contains(got, fragment) {
					t.Fatalf("notice %q does not mention %q", got, fragment)
				}
			}
			if strings.HasPrefix(got, "\n") {
				t.Fatal("the notice must not frame itself with a leading blank line")
			}
		})
	}
}
