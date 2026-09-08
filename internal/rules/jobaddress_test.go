package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWorktreeJobAddressesOffsetsEveryPortAndPublishesTheNamedOnes(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		{Name: "worker", Ports: map[string]int{"METRICS": 9100}},
	}}

	addresses := WorktreeJobAddresses(WorktreeJobAddressesParams{
		Config: cfg, PortOffset: 10, Worktree: "feat-x", Project: "wtm", PublicPort: 8080,
	})

	if got := addresses["web"].Ports; len(got) != 1 || got[0] != 3010 {
		t.Errorf("web ports = %v, want the declared port plus the worktree's offset", got)
	}
	if addresses["web"].URL == "" {
		t.Error("a job declaring a url must carry one")
	}
	if got := addresses["worker"].Ports; len(got) != 1 || got[0] != 9110 {
		t.Errorf("worker ports = %v, want the offset applied to every declared port", got)
	}
	if addresses["worker"].URL != "" {
		t.Error("a job publishing no name has no url, and inventing one would lie")
	}
}

func TestWorktreeJobAddressesSortsPortsSoTheyNeverMoveBetweenTwoReads(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Ports: map[string]int{"B": 4000, "A": 3000}},
	}}

	for range 20 {
		got := WorktreeJobAddresses(WorktreeJobAddressesParams{Config: cfg})["web"].Ports
		if len(got) != 2 || got[0] != 3000 || got[1] != 4000 {
			t.Fatalf("ports = %v, want them sorted", got)
		}
	}
}

func TestWorktreeJobAddressesAnswersNothingForAProjectWithNoRun(t *testing.T) {
	if got := WorktreeJobAddresses(WorktreeJobAddressesParams{}); got != nil {
		t.Fatalf("addresses = %v, want none where nothing is declared", got)
	}
}

func TestWorktreeJobAddressesGivesARunnerTheAddressesOfWhatItHolds(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Runs: []string{"web", "api"}},
		{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		{Name: "api", Ports: map[string]int{"API_PORT": 4000}},
	}}

	addresses := WorktreeJobAddresses(WorktreeJobAddressesParams{
		Config: cfg, PortOffset: 10, Worktree: "feat-x", Project: "wtm", PublicPort: 8080,
	})

	// The ports are the ones the runner's own process was given, so its row
	// stops being the only empty one in a panel where it is the only job up.
	dev := addresses["dev"]
	if len(dev.Ports) != 2 || dev.Ports[0] != 3010 || dev.Ports[1] != 4010 {
		t.Errorf("dev ports = %v, want the ports of the jobs it runs", dev.Ports)
	}
	if len(dev.Held) != 1 || dev.Held[0].Job != "web" {
		t.Fatalf("dev held = %+v, want the one child that publishes a name", dev.Held)
	}
	if dev.Held[0].URL != addresses["web"].URL {
		t.Errorf("held url = %q, want the very address the child answers on (%q)",
			dev.Held[0].URL, addresses["web"].URL)
	}
	if dev.URL != "" {
		t.Errorf("dev url = %q, want none: the runner publishes nothing of its own", dev.URL)
	}
}

// A runner's one line counts the addresses it answers for rather than listing
// them: six urls joined behind a "," ran past the panel and were cut, taking
// four of them with it. They are rows of their own now — never the ports, which
// are its children's and would name the runner after jobs it is not.
func TestJobAddressTextReadsAsTheNamesARunnerHolds(t *testing.T) {
	address := domain.JobAddress{
		Ports: []int{3010, 4010},
		Held: []domain.JobURLEntry{
			{Job: "web", URL: "http://web.wtm"},
			{Job: "api", URL: "http://api.wtm"},
		},
	}

	if got := JobAddressText(address); got != "2 addresses" {
		t.Errorf("text = %q, want the count of what the runner answers for", got)
	}
}
