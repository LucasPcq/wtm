package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func namedPlan(entries ...domain.EnvPortEntry) domain.EnvPortPlan {
	return domain.EnvPortPlan{Addressing: domain.AddressingNames, Entries: entries}
}

func TestPendingOriginRewritesCountsOnlyTheNamedOnes(t *testing.T) {
	plan := namedPlan(
		domain.EnvPortEntry{Key: "CORS_ORIGIN", Addressing: domain.AddressingNames, Status: domain.EnvPortStatusRewrite},
		domain.EnvPortEntry{Key: "VITE_API_URL", Addressing: domain.AddressingNames, Status: domain.EnvPortStatusRewrite},
		// A bare port under the same project: it moves, but no name is at stake.
		domain.EnvPortEntry{Key: "PORT", Addressing: domain.AddressingPorts, Status: domain.EnvPortStatusRewrite},
		domain.EnvPortEntry{Key: "BETTER_AUTH_URL", Addressing: domain.AddressingNames, Status: domain.EnvPortStatusUnchanged},
	)

	if got := PendingOriginRewrites(plan); got != 2 {
		t.Errorf("pending = %d, want the two origins still spelled by port", got)
	}
}

// A key following two jobs may take an origin from one and a bare port from the
// other. Reading the merged entry's addressing off the last link alone counted
// such a value as a port move, and the worktree went unwarned.
func TestPendingOriginRewritesSeesAnOriginFoldedWithAPort(t *testing.T) {
	links := []domain.EnvPortLink{
		{File: ".env", Key: "SERVICES", Job: "api-dev", Port: "PORT"},
		{File: ".env", Key: "SERVICES", Job: "worker", Port: "PORT"},
	}
	jobs := map[string]domain.JobConfig{
		"api-dev": {Name: "api-dev", Ports: map[string]int{"PORT": 4001}, URL: &domain.JobURLConfig{Port: "PORT"}},
		"worker":  {Name: "worker", Ports: map[string]int{"PORT": 5001}},
	}

	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links: links,
		Bases: map[domain.PortRef]int{
			{Job: "api-dev", Name: "PORT"}: 4001,
			{Job: "worker", Name: "PORT"}:  5001,
		},
		Offset: 10,
		Block:  10,
		Lines: map[string][]domain.EnvLine{
			".env": {{Kind: domain.EnvLinePair, Key: "SERVICES", Value: "http://localhost:4001,http://localhost:5001"}},
		},
		Origins: OriginContext{
			Addressing: domain.AddressingNames,
			Jobs:       jobs,
			Worktree:   "feat-x",
			Project:    "repo",
			PublicPort: 11080,
		},
	})

	if got := PendingOriginRewrites(plan); got != 1 {
		t.Errorf("pending = %d, want the origin the port link folded over: %+v", got, plan.Entries)
	}
}

func TestAddressingDriftLineNamesTheWorktreeAndTheCommand(t *testing.T) {
	line := AddressingDriftLine(AddressingDriftParams{
		Worktree: "main",
		Plan: namedPlan(domain.EnvPortEntry{
			Key: "CORS_ORIGIN", Addressing: domain.AddressingNames, Status: domain.EnvPortStatusRewrite,
		}),
	})

	if !strings.Contains(line, "main") || !strings.Contains(line, "wtm env main") {
		t.Errorf("line = %q, want the worktree and the command that aligns it", line)
	}
}

func TestAddressingDriftSaysNothingWhenAligned(t *testing.T) {
	aligned := AddressingDriftParams{
		Worktree: "main",
		Plan: namedPlan(domain.EnvPortEntry{
			Key: "CORS_ORIGIN", Addressing: domain.AddressingNames, Status: domain.EnvPortStatusUnchanged,
		}),
	}

	if got := AddressingDriftLine(aligned); got != "" {
		t.Errorf("line = %q, want nothing for a worktree already addressed by name", got)
	}
	if got := AddressingDriftLines([]AddressingDriftParams{aligned}); got != nil {
		t.Errorf("lines = %+v, want none", got)
	}
}

// A project on ports never reaches the origin path, so its rewrites — every one
// of them a port move — must not be read as a drift.
func TestAddressingDriftIgnoresAProjectOnPorts(t *testing.T) {
	plan := domain.EnvPortPlan{
		Addressing: domain.AddressingPorts,
		Entries: []domain.EnvPortEntry{
			{Key: "PORT", Addressing: domain.AddressingPorts, Status: domain.EnvPortStatusRewrite},
		},
	}

	if got := AddressingDriftLine(AddressingDriftParams{Worktree: "feat/x", Plan: plan}); got != "" {
		t.Errorf("line = %q, want nothing when the project addresses by port", got)
	}
}

func TestAddressingDriftLinesNameEachDriftingWorktreeOnce(t *testing.T) {
	drifting := domain.EnvPortEntry{
		Key: "CORS_ORIGIN", Addressing: domain.AddressingNames, Status: domain.EnvPortStatusRewrite,
	}
	lines := AddressingDriftLines([]AddressingDriftParams{
		{Worktree: "main", Plan: namedPlan(drifting)},
		{Worktree: "feat/x", Plan: namedPlan(drifting)},
		{Worktree: "feat/aligned", Plan: namedPlan()},
	})

	if len(lines) != 2 {
		t.Fatalf("lines = %+v, want one per drifting worktree", lines)
	}
	if !strings.Contains(lines[0], "main") || !strings.Contains(lines[1], "feat/x") {
		t.Errorf("lines = %+v, want each drifting worktree named", lines)
	}
}

func TestAddressedByPortReadsTheValueTheFileHolds(t *testing.T) {
	ported := namedPlan(domain.EnvPortEntry{
		Key: "VITE_API_URL", Addressing: domain.AddressingNames, Status: domain.EnvPortStatusRewrite,
		CurrentValue: "http://localhost:4001",
	})
	if !AddressedByPort(ported) {
		t.Error("a value spelled as a loopback port is what the app answers on, and was not read as one")
	}

	// The .env has moved to names; only the public port went stale, which
	// `proxy install` does to every worktree at once.
	stale := namedPlan(domain.EnvPortEntry{
		Key: "VITE_API_URL", Addressing: domain.AddressingNames, Status: domain.EnvPortStatusRewrite,
		CurrentValue: "http://api-dev.main.repo.localhost:11080",
	})
	if AddressedByPort(stale) {
		t.Error("a stale named origin was read as a port, which would take back the names the .env holds")
	}
}

func TestAddressingDriftLineSeparatesThePortedFromTheStale(t *testing.T) {
	ported := AddressingDriftLine(AddressingDriftParams{Worktree: "main", Plan: namedPlan(domain.EnvPortEntry{
		Addressing: domain.AddressingNames, Status: domain.EnvPortStatusRewrite,
		CurrentValue: "http://localhost:4001",
	})})
	stale := AddressingDriftLine(AddressingDriftParams{Worktree: "main", Plan: namedPlan(domain.EnvPortEntry{
		Addressing: domain.AddressingNames, Status: domain.EnvPortStatusRewrite,
		CurrentValue: "http://api-dev.main.repo.localhost:11080",
	})})

	if ported == stale {
		t.Fatalf("both states got the same sentence: %q", ported)
	}
	if !strings.Contains(ported, "wtm env main") || !strings.Contains(stale, "wtm env main") {
		t.Errorf("one of them names no command: %q / %q", ported, stale)
	}
}

func TestMainAddressing(t *testing.T) {
	entry := func(status domain.EnvPortStatus, current, next string) domain.EnvPortEntry {
		return domain.EnvPortEntry{File: ".env", Key: "API_URL", Addressing: domain.AddressingNames, Status: status, CurrentValue: current, NewValue: next}
	}
	named := "http://api.myrepo.localhost:11080"
	port := "http://localhost:4001"
	cases := []struct {
		name        string
		names       domain.EnvPortEntry
		ports       domain.EnvPortEntry
		wantCurrent domain.Addressing
		wantMatters bool
	}{
		{"on ports", entry(domain.EnvPortStatusRewrite, port, named), entry(domain.EnvPortStatusUnchanged, port, ""), domain.AddressingPorts, true},
		{"on names", entry(domain.EnvPortStatusUnchanged, named, ""), entry(domain.EnvPortStatusRewrite, named, port), domain.AddressingNames, true},
		{"nothing to choose", entry(domain.EnvPortStatusUnchanged, "4001", ""), entry(domain.EnvPortStatusUnchanged, "4001", ""), domain.AddressingNames, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			current, matters := MainAddressing(MainAddressingParams{
				Names: domain.EnvPortPlan{Entries: []domain.EnvPortEntry{tc.names}},
				Ports: domain.EnvPortPlan{Entries: []domain.EnvPortEntry{tc.ports}},
			})
			if current != tc.wantCurrent || matters != tc.wantMatters {
				t.Errorf("got (%s, %v), want (%s, %v)", current, matters, tc.wantCurrent, tc.wantMatters)
			}
		})
	}
}

func TestValidateEnvAddressing(t *testing.T) {
	cases := []struct {
		name   string
		params EnvAddressingParams
		want   error
	}{
		{"unset", EnvAddressingParams{Project: domain.AddressingNames}, nil},
		{"the project's mode on a linked worktree", EnvAddressingParams{Requested: domain.AddressingNames, Project: domain.AddressingNames}, nil},
		{"ports on main", EnvAddressingParams{Requested: domain.AddressingPorts, Project: domain.AddressingNames, IsMain: true}, nil},
		{"ports on a linked worktree", EnvAddressingParams{Requested: domain.AddressingPorts, Project: domain.AddressingNames}, domain.ErrEnvAddressingMainOnly},
		{"names on a ports project", EnvAddressingParams{Requested: domain.AddressingNames, Project: domain.AddressingPorts, IsMain: true}, domain.ErrEnvAddressingProjectPorts},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateEnvAddressing(tc.params); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
