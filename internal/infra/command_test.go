package infra

import (
	"bufio"
	"bytes"
	"context"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type cancelledRun struct {
	Output  string
	Elapsed time.Duration
}

func runCancelled(t *testing.T, script string) cancelledRun {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	var out bytes.Buffer
	cmd := Command(ctx, "sh", "-c", script)
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	begin := time.Now()
	cancel()
	_ = cmd.Wait()
	return cancelledRun{Output: out.String(), Elapsed: time.Since(begin)}
}

// A git or a hook interrupted the way a terminal would interrupt it gets to
// clean up after itself: its lock file, its half-written ref.
func TestCommandInterruptsFirst(t *testing.T) {
	run := runCancelled(t, `trap 'echo int; exit 3' INT; while :; do sleep 0.05; done`)
	if !strings.Contains(run.Output, "int") {
		t.Fatalf("the child never saw SIGINT: %q", run.Output)
	}
	if run.Elapsed >= domain.SubprocessInterruptGrace {
		t.Fatalf("the cancel took %v, want it well under the grace", run.Elapsed)
	}
}

func TestCommandTerminatesAChildThatIgnoresTheInterrupt(t *testing.T) {
	run := runCancelled(t, `trap '' INT; trap 'echo term; exit 4' TERM; while :; do sleep 0.05; done`)
	if !strings.Contains(run.Output, "term") {
		t.Fatalf("the child never saw SIGTERM: %q", run.Output)
	}
}

// A child that runs in its own group takes its whole tree with it: killing the
// shell alone would leave what it started holding the run's pipes.
func TestGroupCommandSignalsTheWholeGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cmd := GroupCommand(ctx, "sh", "-c", `sleep 30 & echo $!; wait`)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	grandchild, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}

	cancel()
	_ = cmd.Wait()

	deadline := time.Now().Add(domain.SubprocessInterruptGrace)
	for syscall.Kill(grandchild, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(grandchild, syscall.SIGKILL)
			t.Fatal("the grandchild outlived the cancelled group")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCommandWithALiveContextRunsToTheEnd(t *testing.T) {
	out, err := Command(t.Context(), "sh", "-c", "echo done").Output()
	if err != nil || strings.TrimSpace(string(out)) != "done" {
		t.Fatalf("Command = %q, %v", out, err)
	}
}
