package seam

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// A Set is built here rather than through OpenSet: the daemon is what Open
// reaches for, and what this is about is the fan-out above it.
func newSet(seams ...Seam) Set { return Set{seams: seams} }

func worktreeSeam(service runlogs.Service, dir, branch string) Seam {
	return Seam{service: service, workDir: dir, worktree: branch}
}

var web = domain.JobConfig{Name: "web", Kind: domain.JobKindService}

// countingSink is deliberately unsynchronised: N sequences reporting at once
// have to be serialised before they reach it, and the race detector is what
// says whether they were.
type countingSink struct {
	events []runlogs.Event
}

func (s *countingSink) Emit(event runlogs.Event) { s.events = append(s.events, event) }

func TestASetReportsOneOutcomePerWorktreeInSelectionOrder(t *testing.T) {
	set := newSet(
		worktreeSeam(&runlogstest.Service{}, "/work/main", "main"),
		worktreeSeam(&runlogstest.Service{}, "/work/feature", "feature"),
	)

	sink := &countingSink{}
	outcomes, err := set.Starter(StartParams{Profile: "dev", Jobs: []domain.JobConfig{web}})(context.Background(), sink)
	if err != nil {
		t.Fatalf("Starter: %v", err)
	}

	if len(outcomes) != 2 {
		t.Fatalf("outcomes = %d, want one per worktree", len(outcomes))
	}
	if outcomes[0].WorkDir != "/work/main" || outcomes[0].Worktree != "main" {
		t.Errorf("first outcome reads as %+v, want the first worktree selected", outcomes[0])
	}
	if outcomes[1].WorkDir != "/work/feature" || outcomes[1].Worktree != "feature" {
		t.Errorf("second outcome reads as %+v, want the second worktree selected", outcomes[1])
	}
}

func TestASetStampsEveryEventWithItsWorktree(t *testing.T) {
	set := newSet(
		worktreeSeam(&runlogstest.Service{}, "/work/main", "main"),
		worktreeSeam(&runlogstest.Service{}, "/work/feature", "feature"),
	)

	sink := &countingSink{}
	if _, err := set.Starter(StartParams{Jobs: []domain.JobConfig{web}})(context.Background(), sink); err != nil {
		t.Fatalf("Starter: %v", err)
	}

	seen := map[string]bool{}
	for _, event := range sink.events {
		if event.WorkDir == "" {
			t.Fatalf("an event came through unstamped: %+v", event)
		}
		seen[event.WorkDir] = true
	}
	if len(seen) != 2 {
		t.Fatalf("events came from %d worktrees, want both", len(seen))
	}
}

// The sinks of N sequences are serialised by the set, never by the surface. The
// barrier makes the two sequences genuinely overlap, so the race detector has
// something to say if they were not.
func TestASetSerialisesWhatTheWorktreesReport(t *testing.T) {
	var barrier sync.WaitGroup
	barrier.Add(2)
	arrive := func(string) {
		barrier.Done()
		barrier.Wait()
	}

	set := newSet(
		worktreeSeam(&runlogstest.Service{Starting: arrive}, "/work/main", "main"),
		worktreeSeam(&runlogstest.Service{Starting: arrive}, "/work/feature", "feature"),
	)

	sink := &countingSink{}
	if _, err := set.Starter(StartParams{Jobs: []domain.JobConfig{web}})(context.Background(), sink); err != nil {
		t.Fatalf("Starter: %v", err)
	}
	if len(sink.events) == 0 {
		t.Fatal("nothing reached the sink")
	}
}

// The worktrees are isolated by construction; an isolation that propagates a
// failure is not one.
func TestAWorktreeThatAbortsLeavesTheOthersAlone(t *testing.T) {
	failing := &runlogstest.Service{Refusals: map[string]string{"web": "boom"}}
	set := newSet(
		worktreeSeam(failing, "/work/main", "main"),
		worktreeSeam(&runlogstest.Service{}, "/work/feature", "feature"),
	)

	outcomes, err := set.Starter(StartParams{Jobs: []domain.JobConfig{web}})(context.Background(), &countingSink{})
	if err != nil {
		t.Fatalf("Starter: %v", err)
	}

	if !outcomes.Aborted() {
		t.Fatal("a set holding an aborted worktree must read as aborted")
	}
	if outcomes[0].Failed != "web" {
		t.Errorf("the failing worktree concluded %+v, want its job named", outcomes[0])
	}
	if outcomes[1].Aborted() || len(outcomes[1].Started) != 1 {
		t.Errorf("the other worktree concluded %+v, want it started regardless", outcomes[1])
	}
}

// The bind port and the port a name answers on diverge exactly when the
// redirection on the 80 is installed. A board built from the bind port then
// announces urls nobody can reach, which is how the logs preview came to
// contradict the header above it.
func TestABoardBuildsItsAddressesFromThePublicPortNotTheBindPort(t *testing.T) {
	published := domain.JobConfig{
		Name:  "web",
		Kind:  domain.JobKindService,
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}

	addresses := boardAddresses(boardAddressParams{
		Params: Params{
			ProjectDir: "/work/shop",
			Jobs:       []domain.JobConfig{published},
			ProxyPort:  8787,
			PublicPort: 80,
		},
		Env: map[string]string{domain.EnvWorktree: "dev-crm", domain.EnvPortOffset: "0"},
	})

	url := addresses[published.Name].URL
	if url == "" {
		t.Fatalf("web has no url, want the name the proxy publishes")
	}
	if strings.Contains(url, "8787") {
		t.Errorf("url = %q, want the public port and never the bind port", url)
	}
}

// PortAddressed is the one worktree fact a board cannot read for itself: the
// name is published, but the .env still answers on the port, so handing out the
// name would hand out an address that fails.
func TestAPortAddressedWorktreeIsHandedItsPortsRatherThanItsNames(t *testing.T) {
	published := domain.JobConfig{
		Name:  "web",
		Kind:  domain.JobKindService,
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}

	addresses := boardAddresses(boardAddressParams{
		Params: Params{
			ProjectDir:    "/work/shop",
			Jobs:          []domain.JobConfig{published},
			PublicPort:    80,
			PortAddressed: true,
		},
		Env: map[string]string{domain.EnvWorktree: "dev-crm", domain.EnvPortOffset: "0"},
	})

	// Not an absent address: a plain port url is the entrance that works, and
	// handing out a name nothing answers to would be worse than handing out none.
	url := addresses[published.Name].URL
	if !strings.Contains(url, "3000") {
		t.Errorf("url = %q, want the port the .env actually answers on", url)
	}
	if strings.Contains(url, "dev-crm") {
		t.Errorf("url = %q, want no published name while the worktree answers on its ports", url)
	}
}
