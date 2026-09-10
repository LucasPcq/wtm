package process

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// unknownStacks is what every test that is not about compose installs: a probe
// that could tell nothing, which is the answer a machine without docker gives and
// the one that leaves R1's behaviour exactly as it was. Without it a unit test
// would shell out to the real docker and answer differently on every machine.
type unknownStacks struct{ asked []StackQuery }

func (s *unknownStacks) Probe(queries []StackQuery) map[string]StackState {
	s.asked = append(s.asked, queries...)
	return nil
}

type verifiedStacks struct {
	up    bool
	asked []StackQuery
}

func (s *verifiedStacks) Probe(queries []StackQuery) map[string]StackState {
	s.asked = append(s.asked, queries...)
	states := make(map[string]StackState, len(queries))
	for _, query := range queries {
		states[query.Key] = StackState{Known: true, Up: s.up}
	}
	return states
}

func TestAdoptReportsAStackVerifiedGoneAsStopped(t *testing.T) {
	dir := t.TempDir()
	index := &recordingIndex{}
	stacks := &verifiedStacks{up: false}

	manager := NewManagerWith(ManagerParams{Index: index, Stacks: stacks})
	manager.Adopt([]domain.JobRecord{detachedRecord(t, dir)})

	jobs := manager.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusStopped {
		t.Fatalf("jobs = %+v, want one reported %q: run ps must not name a stack nobody can reach", jobs, domain.JobStatusStopped)
	}
	if last := index.saved[len(index.saved)-1]; len(last) != 0 {
		t.Fatalf("index still holds %d records: a stack that ended is not up", len(last))
	}
}

func TestAdoptKeepsAStackVerifiedUpDetached(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWith(ManagerParams{Stacks: &verifiedStacks{up: true}})

	manager.Adopt([]domain.JobRecord{detachedRecord(t, dir)})

	jobs := manager.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusDetached {
		t.Fatalf("jobs = %+v, want it still %q", jobs, domain.JobStatusDetached)
	}
}

func TestAdoptAsksTheProbeWhereTheLauncherActuallyRan(t *testing.T) {
	dir := t.TempDir()
	record := detachedRecord(t, dir)
	record.Config.Cwd = "infra"
	stacks := &unknownStacks{}

	manager := NewManagerWith(ManagerParams{Stacks: stacks})
	manager.Adopt([]domain.JobRecord{record})

	if len(stacks.asked) != 1 {
		t.Fatalf("asked %d queries, want the detached entry", len(stacks.asked))
	}
	if want := dir + "/infra"; stacks.asked[0].Dir != want {
		t.Fatalf("dir = %q, want %q: a relative cwd resolved elsewhere asks about another project", stacks.asked[0].Dir, want)
	}
	if stacks.asked[0].Env[domain.EnvComposeProjectName] == "" {
		t.Fatal("the probe must carry COMPOSE_PROJECT_NAME: it is what names the project to ask about")
	}
}

func TestAdoptAsksNothingAboutAForegroundServiceOrAClaim(t *testing.T) {
	dir := t.TempDir()
	claim := detachedRecord(t, dir)
	claim.Attached = true
	claim.SharedDir = dir
	foreground := detachedRecord(t, dir)
	foreground.Name = "dev"
	foreground.Config.Name = "dev"
	foreground.Config.Stop = ""
	stacks := &unknownStacks{}

	manager := NewManagerWith(ManagerParams{Stacks: stacks})
	manager.Adopt([]domain.JobRecord{claim, foreground})

	if len(stacks.asked) != 0 {
		t.Fatalf("asked %+v: a claim owns no launcher, and a foreground service is the other probe's subject", stacks.asked)
	}
}

func TestComposeProbeSurvivesAMachineWithoutDocker(t *testing.T) {
	// The real prober, against a directory holding no compose file and a project
	// nothing ever created. Whatever the machine answers — docker missing, an
	// engine that is down, a refusal — the contract is that nothing is claimed.
	states := systemStacks{}.Probe([]StackQuery{{
		Key: "k",
		Job: domain.JobConfig{Kind: domain.JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"},
		Dir: t.TempDir(),
		Env: map[string]string{domain.EnvComposeProjectName: "wtm-does-not-exist-" + t.Name()},
	}})

	if state := states["k"]; state.Known && state.Up {
		t.Fatalf("state = %+v: nothing was ever started, so nothing may be reported up", state)
	}
}
