package process

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// stressDuration is how long TestManagerUnderConcurrentUse_HasNoRace hammers
// the manager. The window between a descriptor being closed and the closer
// blanking it is a handful of instructions wide, so the race detector needs
// enough passes to land inside it.
const stressDuration = 1500 * time.Millisecond

type stressWork struct {
	Count int
	Do    func(worker int)
}

type stressGroup struct {
	wg   sync.WaitGroup
	stop chan struct{}
}

func newStressGroup() *stressGroup {
	return &stressGroup{stop: make(chan struct{})}
}

func (g *stressGroup) spawn(work stressWork) {
	for i := range work.Count {
		g.wg.Add(1)
		go func(worker int) {
			defer g.wg.Done()
			for {
				select {
				case <-g.stop:
					return
				default:
				}
				work.Do(worker)
			}
		}(i)
	}
}

func (g *stressGroup) runFor(d time.Duration) {
	time.Sleep(d)
	close(g.stop)
	g.wg.Wait()
}

// TestManagerUnderConcurrentUse_HasNoRace drives every entry point a run view
// reaches at once — panes resizing and attaching while services crash, restart
// and are stopped under them. Each of those paths is race-free on its own, so
// only running them together pins what matters: a job's PTY is closed by
// whichever goroutine reaps or stops it, concurrently with the panes still
// sizing it, and the fields a lister reads are written by those same reapers.
func TestManagerUnderConcurrentUse_HasNoRace(t *testing.T) {
	m := newManager()
	dir := t.TempDir()

	crash := filepath.Join(dir, "crash.sh")
	if err := os.WriteFile(crash, []byte("sleep 0.05; exit 3\n"), 0o755); err != nil {
		t.Fatalf("write crash script: %v", err)
	}

	if err := m.Start(StartParams{
		Job:     domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 30"},
		WorkDir: dir,
	}); err != nil {
		t.Fatalf("start dev: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	crasher := func(worker int) string { return fmt.Sprintf("svc%d", worker) }
	g := newStressGroup()

	g.spawn(stressWork{Count: 4, Do: func(worker int) {
		_ = m.Start(StartParams{
			Job:     domain.JobConfig{Name: crasher(worker), Kind: domain.JobKindService, Cmd: "sh " + crash},
			WorkDir: dir,
		})
		time.Sleep(time.Millisecond)
	}})

	g.spawn(stressWork{Count: 1, Do: func(int) {
		_ = m.Start(StartParams{
			Job:     domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "echo hi"},
			WorkDir: dir,
		})
	}})

	g.spawn(stressWork{Count: 4, Do: func(int) {
		for _, job := range m.List() {
			_, _, _, _ = job.Status, job.ExitCode, job.StartedAt, job.PID
		}
		_ = m.IsRunning()
	}})

	g.spawn(stressWork{Count: 3, Do: func(worker int) {
		for _, name := range []string{"dev", crasher(worker), "migrate"} {
			_ = m.Resize(ResizeParams{Name: name, WorkDir: dir, Cols: 80, Rows: 24})
		}
	}})

	g.spawn(stressWork{Count: 3, Do: func(worker int) {
		for _, name := range []string{"dev", crasher(worker)} {
			session, err := m.Attach(JobRef{Name: name, WorkDir: dir})
			if err != nil {
				continue
			}
			session.Release()
		}
	}})

	g.spawn(stressWork{Count: 1, Do: func(int) {
		_ = m.Stop(JobRef{Name: "svc0", WorkDir: dir})
		time.Sleep(2 * time.Millisecond)
	}})

	g.runFor(stressDuration)
}
