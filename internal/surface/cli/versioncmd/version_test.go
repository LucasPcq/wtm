package versioncmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func run(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd := NewCmd(NewCmdParams{Version: "1.2.3"})
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// The shape is a contract hosts branch on: a key renamed or dropped is a host
// that can no longer tell which wtm it talks to.
func TestTheJSONReportsTheVersionAndEveryContract(t *testing.T) {
	var got map[string]any
	if err := json.Unmarshal([]byte(run(t, "--"+domain.FlagOutput, domain.OutputJSON)), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"version": "1.2.3", "events": float64(domain.EventsSchemaVersion)}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("report = %v, want %v", got, want)
	}
}

func TestTheTextIsTheVersionLine(t *testing.T) {
	if got, want := run(t), "wtm version 1.2.3\n"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}
