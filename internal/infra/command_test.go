package infra

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
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

// git does not stop the ssh it started when it is stopped itself: the
// cancellation reaches what the child started too.
func TestCommandStopsWhatTheChildStarted(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cmd := Command(ctx, "sh", "-c", `sleep 30 & echo $!; wait`)
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
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })

	cancel()
	_ = cmd.Wait()

	// A shell's background job ignores SIGINT: it goes on the SIGTERM a grace later.
	deadline := time.Now().Add(2 * domain.SubprocessInterruptGrace)
	for syscall.Kill(grandchild, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatal("the grandchild outlived the cancelled child")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

const daemonHelperEnv = "WTM_TEST_DAEMON_HELPER"

// TestDaemonHelper is the daemon a git child may leave behind: a credential
// cache, an fsmonitor, an ssh ControlPersist master. It leaves its parent's
// session the way they all do.
func TestDaemonHelper(t *testing.T) {
	if os.Getenv(daemonHelperEnv) == "" {
		t.Skip("helper process")
	}
	if _, err := syscall.Setsid(); err != nil {
		os.Exit(2)
	}
	fmt.Println(os.Getpid())
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

// A cancellation stops what the child started, never a daemon it left behind
// on purpose: killing a credential cache or an ssh master on Ctrl-C would cost
// the user their setup.
func TestCommandLeavesADaemonTheChildStartedAlone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	script := fmt.Sprintf(`%s=1 %q -test.run='^TestDaemonHelper$' & sleep 30 & wait`, daemonHelperEnv, os.Args[0])
	cmd := Command(ctx, "sh", "-c", script)
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
	daemon, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("helper said %q", line)
	}
	t.Cleanup(func() { _ = syscall.Kill(daemon, syscall.SIGKILL) })

	cancel()
	_ = cmd.Wait()
	KillCancelled()
	time.Sleep(200 * time.Millisecond)

	if syscall.Kill(daemon, 0) != nil {
		t.Error("the daemon the child left behind was killed with it")
	}
}
