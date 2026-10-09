package env

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func checkPortsParams(main, wt string) SyncEnvParams {
	return SyncEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeAdd,
		Ports: EnvPortsParams{
			WorktreePath: wt,
			Links:        []domain.EnvPortLink{{File: ".env", Key: "API_URL", Job: "api", Port: "PORT"}},
			Bases:        map[domain.PortRef]int{{Job: "api", Name: "PORT"}: 3000},
			Offset:       10,
			Block:        10,
		},
	}
}

func portStatuses(plan domain.EnvPortPlan) map[string]domain.EnvPortStatus {
	out := map[string]domain.EnvPortStatus{}
	for _, entry := range plan.Entries {
		out[entry.Key] = entry.Status
	}
	return out
}

// LUC-276 (b): a --check over a .env the worktree lacks reported every port key
// as missing, while the apply rebuilt the file and settled them.
func TestSyncCheckPlansPortsOnWhatTheApplyWrites(t *testing.T) {
	cases := map[string]string{
		"file missing":            "",
		"key the apply would add": "OTHER=1\n",
	}
	for name, worktreeEnv := range cases {
		t.Run(name, func(t *testing.T) {
			main := t.TempDir()
			writeTestFile(t, filepath.Join(main, ".env"), "API_URL=http://localhost:3000\nOTHER=1\n")

			checkDir := t.TempDir()
			applyDir := t.TempDir()
			if worktreeEnv != "" {
				writeTestFile(t, filepath.Join(checkDir, ".env"), worktreeEnv)
				writeTestFile(t, filepath.Join(applyDir, ".env"), worktreeEnv)
			}

			checkParams := checkPortsParams(main, checkDir)
			checkParams.Check = true
			checked, err := SyncEnv(checkParams)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			applied, err := SyncEnv(checkPortsParams(main, applyDir))
			if err != nil {
				t.Fatalf("apply: %v", err)
			}

			got, want := portStatuses(checked.Ports), portStatuses(applied.Ports)
			if got["API_URL"] != want["API_URL"] || want["API_URL"] == domain.EnvPortStatusMissingKey {
				t.Fatalf("check plans API_URL as %q, the apply as %q", got["API_URL"], want["API_URL"])
			}
			if fileExists(filepath.Join(checkDir, ".env")) != (worktreeEnv != "") {
				t.Fatal("--check wrote the .env")
			}
		})
	}
}
