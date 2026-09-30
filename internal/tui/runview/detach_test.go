package runview

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// recorder is a surface's sink: what a run reports once nobody is watching it.
type recorder struct {
	mu     sync.Mutex
	events []runlogs.Event
}

func (r *recorder) Emit(event runlogs.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) jobs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var names []string
	for _, event := range r.events {
		if event.Phase == runlogs.PhaseStarting {
			names = append(names, event.Job)
		}
	}
	return names
}

// stepwiseStart is a profile that starts its jobs one at a time and waits to be
// told to carry on, so a test can leave the view in the middle of the sequence
// exactly as a reader does.
type stepwiseStart struct {
	jobs    []string
	step    chan struct{}
	started chan string
}

func newStepwiseStart(jobs ...string) *stepwiseStart {
	return &stepwiseStart{
		jobs:    jobs,
		step:    make(chan struct{}),
		started: make(chan string, len(jobs)),
	}
}

func (s *stepwiseStart) run(ctx context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
	for i, job := range s.jobs {
		select {
		case <-s.step:
		case <-ctx.Done():
			// The run reports what it was told to abandon, which is exactly what
			// leaving the view must not cause.
			return runlogs.Outcomes{{NotStarted: s.jobs[i:]}}, ctx.Err()
		}
		sink.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: job, Step: i + 1, Steps: len(s.jobs)})
		sink.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: job, Step: i + 1, Steps: len(s.jobs)})
		s.started <- job
	}
	return runlogs.Outcomes{{Started: s.jobs}}, nil
}

func (s *stepwiseStart) next(t *testing.T) string {
	t.Helper()
	select {
	case s.step <- struct{}{}:
	case <-time.After(5 * time.Second):
		t.Fatal("the run never asked for its next job")
	}
	select {
	case job := <-s.started:
		return job
	case <-time.After(5 * time.Second):
		t.Fatal("the run never started its next job")
		return ""
	}
}

func detachModel(t *testing.T, start *stepwiseStart, onLeave Detach) Model {
	t.Helper()
	model := New(Params{
		Board:  runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{stopped("migrate")}}),
		Start:  start.run,
		Detach: onLeave,
	})
	return model
}

// Leaving the view stops the watching, not the run. The sequence belongs to
// this process — the daemon cannot take it over — so cancelling it on the way
// out abandoned every job it had not reached yet: quitting during `migrate`
// meant `seed` and `web` never ran at all.
func TestLeavingTheViewLetsTheRestOfTheProfileStart(t *testing.T) {
	start := newStepwiseStart("migrate", "seed", "web")
	surface := &recorder{}
	handedOver := make(chan struct{})
	model := detachModel(t, start, Detach{
		Notice: func() { close(handedOver) },
		Sink:   surface,
		Await:  true,
	})

	program := newProgram(t, model)
	program.send(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	start.next(t)

	program.mu.Lock()
	left, _ := program.model.detach()
	leaving := left.(Model)
	program.model = leaving
	program.mu.Unlock()

	done := make(chan Result, 1)
	go func() { done <- leaving.awaitDetached() }()

	// The run only advances once the surface has been named. Letting it run
	// ahead would assert the hand-over's hold-and-replay window, which is a
	// different test (TestEventsDuringTheHandOverReachTheSurface).
	select {
	case <-handedOver:
	case <-time.After(5 * time.Second):
		t.Fatal("leaving the view never handed the run to the surface")
	}

	start.next(t)
	start.next(t)

	select {
	case result := <-done:
		if !result.Detached {
			t.Error("the run finished without the view; the result must say so")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("leaving the view never gave back what the run concluded")
	}

	got := surface.jobs()
	if len(got) != 2 || got[0] != "seed" || got[1] != "web" {
		t.Errorf("the surface was told about %v, want the two jobs that started after the reader left", got)
	}
}

// A surface with nowhere to report the rest cannot honestly claim the run
// continues, so leaving cancels it — the behaviour every surface had before.
func TestLeavingCancelsTheRunWhenNoSurfaceTakesItOver(t *testing.T) {
	start := newStepwiseStart("migrate", "seed")
	model := detachModel(t, start, Detach{})

	program := newProgram(t, model)
	program.send(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	start.next(t)

	program.mu.Lock()
	left, _ := program.model.detach()
	leaving := left.(Model)
	program.model = leaving
	program.mu.Unlock()

	if leaving.runCtx.Err() == nil {
		t.Error("nobody was left to report the run and it was not cancelled")
	}
}

// The hand-over is the terminal being given back, and a job can start inside
// that window. Those events are held rather than dropped: a job that started
// while the screen was being restored is exactly what the reader left to allow.
func TestEventsDuringTheHandOverReachTheSurface(t *testing.T) {
	relay := newRelay(&recorder{})
	relay.detach()

	relay.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: "seed"})
	relay.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "seed"})

	surface := &recorder{}
	relay.redirect(surface)

	if got := surface.jobs(); len(got) != 1 || got[0] != "seed" {
		t.Errorf("the surface was told about %v, want the job that started during the hand-over", got)
	}
}

// A job printing without pause must not grow a buffer nobody is reading. Its
// lifecycle events are what the account is built from and are never dropped.
func TestHeldOutputIsCappedButLifecycleEventsAreNot(t *testing.T) {
	relay := newRelay(&recorder{})
	relay.detach()

	for range domain.RunDetachHeldChunks * 4 {
		relay.Emit(runlogs.Event{Phase: runlogs.PhaseOutput, Chunk: []byte("noise\n")})
	}
	relay.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: "web"})

	surface := &recorder{}
	relay.redirect(surface)

	surface.mu.Lock()
	total := len(surface.events)
	surface.mu.Unlock()
	if total > domain.RunDetachHeldChunks+1 {
		t.Errorf("held %d events, want the output capped at %d", total, domain.RunDetachHeldChunks)
	}
	if got := surface.jobs(); len(got) != 1 || got[0] != "web" {
		t.Errorf("the surface was told about %v, want the lifecycle event kept whatever the output did", got)
	}
}

// A surface that outlives the view takes its terminal back at once: the run
// reports into it while it draws, rather than holding it on a dead screen for
// the rest of the profile.
func TestASurfaceThatOutlivesTheViewDoesNotWaitForTheRun(t *testing.T) {
	start := newStepwiseStart("migrate", "seed")
	surface := &recorder{}
	model := detachModel(t, start, Detach{Sink: surface})

	program := newProgram(t, model)
	program.send(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	start.next(t)

	program.mu.Lock()
	left, _ := program.model.detach()
	leaving := left.(Model)
	program.mu.Unlock()

	returned := make(chan Result, 1)
	go func() { returned <- leaving.awaitDetached() }()

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("the view held its surface until the run ended; a dashboard would show a dead screen")
	}

	start.next(t)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got := surface.jobs(); len(got) == 1 && got[0] == "seed" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("the surface was told about %v, want the job that started after it took its terminal back", surface.jobs())
}
