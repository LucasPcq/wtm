package execsvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func targets(t *testing.T, n int) []Target {
	t.Helper()
	out := make([]Target, n)
	for i := range out {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		out[i] = Target{Branch: string(rune('a' + i)), Path: dir, Env: os.Environ(), LogPath: filepath.Join(t.TempDir(), "x.log")}
	}
	return out
}

func TestRunReportsEveryTargetInOrderAndKeepsGoingAfterAFailure(t *testing.T) {
	ts := targets(t, 3)
	results := Run(context.Background(), RunParams{
		Command: `[ "$PWD" = "` + ts[0].Path + `" ] && sleep 0.3; [ "$PWD" = "` + ts[1].Path + `" ] && exit 3; true`,
		Targets: ts,
		Jobs:    3,
	})
	if len(results) != 3 {
		t.Fatalf("results = %d", len(results))
	}
	for i, r := range results {
		if r.Branch != ts[i].Branch {
			t.Errorf("result %d is %q, want %q (target order)", i, r.Branch, ts[i].Branch)
		}
	}
	if results[1].Status != domain.ExecStatusFailed || results[2].Status != domain.ExecStatusPassed {
		t.Errorf("a failure must not stop the others: %+v", results)
	}
}

func TestRunCapturesExitCodeTailAndLog(t *testing.T) {
	ts := targets(t, 2)
	results := Run(context.Background(), RunParams{Command: `echo one; echo two >&2; [ "$PWD" = "` + ts[1].Path + `" ] && exit 7; true`, Targets: ts, Jobs: 2})

	if results[0].Status != domain.ExecStatusPassed || *results[0].ExitCode != 0 {
		t.Fatalf("first = %+v", results[0])
	}
	if results[1].Status != domain.ExecStatusFailed || *results[1].ExitCode != 7 {
		t.Fatalf("second = %+v", results[1])
	}
	if strings.Join(results[1].Tail, ",") != "one,two" {
		t.Errorf("tail = %v", results[1].Tail)
	}
	log, err := os.ReadFile(ts[1].LogPath)
	if err != nil || string(log) != "one\ntwo\n" {
		t.Errorf("log = %q, %v", log, err)
	}
}

func TestRunNeverExceedsTheConcurrencyBound(t *testing.T) {
	var mu sync.Mutex
	running, peak := 0, 0
	ts := targets(t, 6)
	Run(context.Background(), RunParams{
		Command: "sleep 0.2",
		Targets: ts,
		Jobs:    2,
		OnBeat: func(beat domain.ExecBeat) {
			mu.Lock()
			defer mu.Unlock()
			if beat.Started {
				running++
				peak = max(peak, running)
				return
			}
			running--
		},
	})
	if peak != 2 {
		t.Fatalf("peak concurrency = %d, want 2", peak)
	}
}

func TestRunClosesStdin(t *testing.T) {
	results := Run(context.Background(), RunParams{Command: "read line", Targets: targets(t, 1), Jobs: 1})
	if results[0].Status != domain.ExecStatusFailed {
		t.Fatalf("a command reading stdin must fail, not hang: %+v", results[0])
	}
}

func TestRunKeepsTheWholeOutputWhenAsked(t *testing.T) {
	results := Run(context.Background(), RunParams{Command: "seq 1 50", Targets: targets(t, 1), Jobs: 1, KeepOutput: true})
	if !strings.HasPrefix(results[0].Output, "1\n2\n") || !strings.HasSuffix(results[0].Output, "50\n") {
		t.Fatalf("output = %q", results[0].Output)
	}
}

func TestRunUsesTheTargetsEnvironment(t *testing.T) {
	ts := targets(t, 1)
	ts[0].Env = append(os.Environ(), "WTM_EXEC_PROBE=seen")
	results := Run(context.Background(), RunParams{Command: `[ "$WTM_EXEC_PROBE" = seen ]`, Targets: ts, Jobs: 1})
	if results[0].Status != domain.ExecStatusPassed {
		t.Fatalf("env not passed: %+v", results[0])
	}
}

func TestCancelInterruptsRunningAndSkipsQueued(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	go func() {
		<-started
		cancel()
	}()
	results := Run(ctx, RunParams{
		Command: "sleep 30",
		Targets: targets(t, 3),
		Jobs:    1,
		OnBeat: func(beat domain.ExecBeat) {
			if beat.Started {
				select {
				case started <- struct{}{}:
				default:
				}
			}
		},
	})
	if results[0].Status != domain.ExecStatusInterrupted {
		t.Errorf("running = %s, want interrupted", results[0].Status)
	}
	if results[1].Status != domain.ExecStatusNotStarted || results[2].Status != domain.ExecStatusNotStarted {
		t.Errorf("queued = %s, %s, want not_started", results[1].Status, results[2].Status)
	}
}

func TestCancelKillsACommandThatIgnoresSIGINT(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()
	begin := time.Now()
	results := Run(ctx, RunParams{Command: "trap '' INT; sleep 60", Targets: targets(t, 1), Jobs: 1})
	if results[0].Status != domain.ExecStatusInterrupted {
		t.Errorf("status = %s", results[0].Status)
	}
	if elapsed := time.Since(begin); elapsed > domain.ExecInterruptGrace+3*time.Second {
		t.Fatalf("took %s: SIGINT was ignored and nothing escalated", elapsed)
	}
}

func TestAVanishedWorktreeIsAFailedResultNotAPanic(t *testing.T) {
	ts := targets(t, 2)
	ts[0].Path = filepath.Join(t.TempDir(), "gone")
	results := Run(context.Background(), RunParams{Command: "true", Targets: ts, Jobs: 2})
	if results[0].Status != domain.ExecStatusFailed || results[0].Error == "" {
		t.Fatalf("vanished = %+v", results[0])
	}
	if results[1].Status != domain.ExecStatusPassed {
		t.Fatalf("the other target must still run: %+v", results[1])
	}
}
