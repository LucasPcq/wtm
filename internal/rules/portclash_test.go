package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestPortClaimsLeaveSharedJobsOut(t *testing.T) {
	claims := PortClaims(PortClaimsParams{
		Jobs: []domain.JobConfig{
			{Name: "web", Ports: map[string]int{"PORT": 3000}},
			{Name: "db", Ports: map[string]int{"DB_PORT": 5432}, Scope: domain.JobScopeShared},
		},
		WorkDir: "/wt/a",
		Offset:  10,
	})
	if len(claims) != 1 || claims[0].Port != 3010 || claims[0].Job != "web" {
		t.Errorf("claims = %+v, want web alone, on its shifted port", claims)
	}
}

func TestPortClashes(t *testing.T) {
	starting := []domain.PortClaim{
		{Port: 3000, Job: "web", WorkDir: "/wt/verbatim"},
		{Port: 3001, Job: "api", WorkDir: "/wt/verbatim"},
	}
	held := []domain.PortClaim{
		{Port: 3000, Job: "web", WorkDir: "/wt/main"},
		{Port: 3011, Job: "api", WorkDir: "/wt/isolated"},
	}
	clashes := PortClashes(PortClashesParams{Starting: starting, Held: held})
	if len(clashes) != 1 || clashes[0].Port != 3000 || clashes[0].HeldBy.WorkDir != "/wt/main" {
		t.Fatalf("clashes = %+v, want 3000 held by main", clashes)
	}
	if lines := PortClashLines(clashes); len(lines) != 1 {
		t.Errorf("lines = %v, want one", lines)
	}
}

// A runner and its child declare the same port in one worktree: that is not
// two worktrees fighting over it.
func TestSelfPortClashesOnlyAcrossWorktrees(t *testing.T) {
	same := []domain.PortClaim{
		{Port: 3000, Job: "dev", WorkDir: "/wt/a"},
		{Port: 3000, Job: "web", WorkDir: "/wt/a"},
	}
	if clashes := SelfPortClashes(same); len(clashes) != 0 {
		t.Errorf("clashes = %+v, want none inside one worktree", clashes)
	}
	across := append(same, domain.PortClaim{Port: 3000, Job: "web", WorkDir: "/wt/b"})
	if clashes := SelfPortClashes(across); len(clashes) != 1 {
		t.Errorf("clashes = %+v, want the two worktrees on 3000", clashes)
	}
}

func TestDecideConcurrencyOnAClash(t *testing.T) {
	cases := []struct {
		name   string
		params ConcurrencyParams
		want   ConcurrencyDecision
	}{
		{"asked", ConcurrencyParams{OthersRunning: true, Clashes: true}, ConcurrencyDecision{Value: domain.ConcurrencyExclusive, Ask: true, Clash: true}},
		{"parallel cannot be honoured", ConcurrencyParams{OthersRunning: true, Clashes: true, Parallel: true}, ConcurrencyDecision{Value: domain.ConcurrencyExclusive, Ask: true, Clash: true}},
		{"--exclusive", ConcurrencyParams{OthersRunning: true, Clashes: true, Exclusive: true}, ConcurrencyDecision{Value: domain.ConcurrencyExclusive}},
		{"settled exclusive", ConcurrencyParams{OthersRunning: true, Clashes: true, Config: domain.ConcurrencyExclusive}, ConcurrencyDecision{Value: domain.ConcurrencyExclusive}},
	}
	for _, tc := range cases {
		if got := DecideConcurrency(tc.params); got != tc.want {
			t.Errorf("%s: DecideConcurrency = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}
