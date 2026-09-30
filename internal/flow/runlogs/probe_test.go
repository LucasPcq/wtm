package runlogs_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// listening answers a fixed set of ports, standing in for the dial.
type listening map[int]bool

func (l listening) Listening(_ context.Context, ports []int, _ func(map[int]bool) bool) map[int]bool {
	answered := map[int]bool{}
	for _, p := range ports {
		if l[p] {
			answered[p] = true
		}
	}
	return answered
}

var (
	web    = domain.JobConfig{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"WEB_PORT": 5173}}
	seeder = domain.JobConfig{Name: "seed", Kind: domain.JobKindTask, Cmd: "pnpm seed", Ports: map[string]int{"SEED_PORT": 9000}}
)

func runProbed(t *testing.T, prober runlogs.Prober, offset string, jobs ...domain.JobConfig) (runlogs.Outcome, *runlogstest.Sink) {
	t.Helper()
	service := &runlogstest.Service{Ports: map[string]map[string]int{
		"web": {"WEB_PORT": 5183},
		"api": {"API_PORT": 3011},
	}}
	sink := &runlogstest.Sink{}
	outcome, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service: service,
		Sink:    sink,
		Jobs:    jobs,
		Prober:  prober,
		Env:     map[string]string{domain.EnvPortOffset: offset},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return outcome, sink
}

func probesFor(sink *runlogstest.Sink, job string) []domain.PortProbe {
	for _, e := range sink.Events {
		if e.Phase == runlogs.PhaseProbed && e.Job == job {
			return e.Probes
		}
	}
	return nil
}

func TestRunProbesAServiceThatDeclaredPorts(t *testing.T) {
	_, sink := runProbed(t, listening{5183: true}, "10", web)

	probes := probesFor(sink, "web")
	if len(probes) != 1 {
		t.Fatalf("expected 1 probe, got %+v", probes)
	}
	if probes[0].Status != domain.PortListening {
		t.Errorf("status = %s, want listening", probes[0].Status)
	}
	if probes[0].Port != 5183 {
		t.Errorf("probed %d, want the resolved port 5183", probes[0].Port)
	}
}

func TestRunNamesTheBasePortWhenTheVariableDidNotArrive(t *testing.T) {
	// The signature of the whole feature: the resolved port is silent while the
	// base one answers.
	_, sink := runProbed(t, listening{5173: true}, "10", web)

	probes := probesFor(sink, "web")
	if len(probes) != 1 || probes[0].Status != domain.PortSilent {
		t.Fatalf("expected a silent port, got %+v", probes)
	}
	if probes[0].BaseListening != 5173 {
		t.Errorf("BaseListening = %d, want 5173", probes[0].BaseListening)
	}
}

func TestRunNeverProbesATask(t *testing.T) {
	_, sink := runProbed(t, listening{}, "10", seeder)

	if probes := probesFor(sink, "seed"); probes != nil {
		t.Errorf("a task must not be probed, got %+v", probes)
	}
}

func TestRunWithoutAProberEmitsNothing(t *testing.T) {
	_, sink := runProbed(t, nil, "10", web)

	for _, e := range sink.Events {
		if e.Phase == runlogs.PhaseProbed {
			t.Fatalf("no prober installed, yet a probe was emitted: %+v", e)
		}
	}
}

func TestOutcomeCarriesEveryProbe(t *testing.T) {
	outcome, _ := runProbed(t, listening{5183: true}, "10", web)

	if len(outcome.Probes) != 1 {
		t.Fatalf("expected the outcome to carry the probes, got %+v", outcome.Probes)
	}
	if outcome.Probes[0].Job != "web" {
		t.Errorf("probe lost its job: %q", outcome.Probes[0].Job)
	}
}

func TestProbedComesBeforeReady(t *testing.T) {
	_, sink := runProbed(t, listening{5183: true}, "10", web)

	probed, ready := -1, -1
	for i, e := range sink.Events {
		switch e.Phase {
		case runlogs.PhaseProbed:
			probed = i
		case runlogs.PhaseReady:
			ready = i
		}
	}
	if probed < 0 || ready < 0 {
		t.Fatalf("missing a phase: probed=%d ready=%d", probed, ready)
	}
	if probed > ready {
		t.Error("PhaseReady concludes the sequence and must come last")
	}
}
