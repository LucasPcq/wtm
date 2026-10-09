package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// LUC-279: a restored row names the ports run.toml declares for the key, not
// what it read from either value.
func TestEnvRestoredRowsNameTheDeclaredPorts(t *testing.T) {
	rows := EnvRestoredRows(EnvRestoredRowsParams{File: ".env", Entries: []domain.EnvRestoredEntry{{
		File: ".env", Key: "ORIGINS", Ports: []int{3000, 3001},
		From: "http://localhost:3040,http://a:" + fakeSecret + "@localhost:3041",
		To:   "http://localhost:3000,http://a:" + fakeSecret + "@localhost:3001",
	}}})

	if len(rows) != 1 || !strings.HasSuffix(rows[0], "back to the source's :3000, :3001") {
		t.Errorf("rows = %q, want the declared ports", rows)
	}
}

func TestRestoreOwnedEnvCarriesTheDeclaredPortsOfALinkedKey(t *testing.T) {
	_, entries := RestoreOwnedEnv(RestoreOwnedEnvParams{
		File:      ".env",
		Child:     ParseEnv("DB=postgres://localhost:5442/db\nREALM=app-feat\n"),
		Source:    ParseEnv("DB=postgres://localhost:5432/db\nREALM=main\n"),
		Keys:      []string{"DB", "REALM"},
		PortBases: map[string][]int{"DB": {5432}},
	})

	if len(entries) != 2 || !slices.Equal(entries[0].Ports, []int{5432}) || entries[1].Ports != nil {
		t.Errorf("entries = %+v, want DB's declared port and none for REALM", entries)
	}
}

// A restore that also puts back a password edited in the worktree is not a
// port move, and a row naming only the port would hide that the edit is lost.
func TestRestoreOwnedEnvNamesNoPortWhenMoreThanThePortGoesBack(t *testing.T) {
	for _, child := range []string{
		"DB=postgres://app:edited@localhost:5442/db",
		"DB=postgres://app:pw@localhost:5442/db?pin=42424242",
		"DB=postgres://app:pw@localhost:5442/db?pin=48151623x",
	} {
		_, entries := RestoreOwnedEnv(RestoreOwnedEnvParams{
			File:      ".env",
			Child:     ParseEnv(child + "\n"),
			Source:    ParseEnv("DB=postgres://app:pw@localhost:5432/db?pin=48151623\n"),
			Keys:      []string{"DB"},
			PortBases: map[string][]int{"DB": {5432}},
		})
		if len(entries) != 1 || entries[0].Ports != nil {
			t.Errorf("%q: entries = %+v, want no port named", child, entries)
		}
	}
}
