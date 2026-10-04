package events

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	wtmevents "github.com/LucasPcq/wtm/internal/service/events"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// lines hands each line the command writes to the test as it is written.
type lines struct {
	mu      sync.Mutex
	pending bytes.Buffer
	ch      chan string
}

func (l *lines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending.Write(p)
	for {
		line, err := l.pending.ReadString('\n')
		if err != nil {
			l.pending.WriteString(line)
			return len(p), nil
		}
		l.ch <- strings.TrimRight(line, "\n")
	}
}

func (l *lines) next(t *testing.T) string {
	t.Helper()
	select {
	case line := <-l.ch:
		return line
	case <-time.After(5 * time.Second):
		t.Fatal("no line")
		return ""
	}
}

func initializedRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
	if err := os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

type running struct {
	out  *lines
	done chan error
	stop context.CancelFunc
}

func start(t *testing.T, args ...string) running {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r := running{out: &lines{ch: make(chan string, 64)}, done: make(chan error, 1), stop: cancel}
	cmd := NewCmd()
	cmd.SetArgs(args)
	cmd.SetOut(r.out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(ctx)
	go func() { r.done <- cmd.Execute() }()
	return r
}

func (r running) end(t *testing.T) error {
	t.Helper()
	r.stop()
	select {
	case err := <-r.done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the command did not stop")
		return nil
	}
}

func decode(t *testing.T, line string) domain.Event {
	t.Helper()
	var event domain.Event
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("not a JSON line: %q: %v", line, err)
	}
	return event
}

func TestTheStreamOpensOnASnapshotAndCarriesWhatIsPublished(t *testing.T) {
	processtest.Home(t)
	processtest.RealDaemon(t, process.SocketPath())
	dir := initializedRepo(t)
	r := start(t, "--repo", dir, "--"+domain.FlagOutput, domain.OutputJSON)

	if got := decode(t, r.out.next(t)); got.Type != domain.EventSnapshot || got.Repo == nil || len(got.Worktrees) != 1 {
		t.Fatalf("first line = %+v", got)
	}
	if got := decode(t, r.out.next(t)); got.Type != domain.EventReady {
		t.Fatalf("second line = %+v", got)
	}
	wtmevents.NewPublisher(wtmevents.PublisherParams{ProjectDir: dir}).Publish(domain.Event{Type: domain.EventWorktreeCreated, Worktree: &domain.WorktreeIdentity{Branch: "feat/a", Path: "/p"}})
	if got := decode(t, r.out.next(t)); got.Type != domain.EventWorktreeCreated || got.Worktree.Branch != "feat/a" {
		t.Fatalf("third line = %+v", got)
	}

	if err := r.end(t); err != nil {
		t.Fatalf("an interrupted stream is a success: %v", err)
	}
}

func TestTheTextStreamIsOneLinePerEvent(t *testing.T) {
	processtest.Home(t)
	processtest.RealDaemon(t, process.SocketPath())
	dir := initializedRepo(t)
	r := start(t, "--repo", dir)

	if got := r.out.next(t); !strings.Contains(got, "1 worktree ·") {
		t.Fatalf("first line = %q", got)
	}
	if got := r.out.next(t); !strings.Contains(got, domain.EventReadyMessage) {
		t.Fatalf("second line = %q", got)
	}
	_ = r.end(t)
}

func TestANewerSchemaExitsWithItsOwnCode(t *testing.T) {
	processtest.Home(t)
	processtest.RealDaemon(t, process.SocketPath())
	dir := initializedRepo(t)
	r := start(t, "--repo", dir, "--"+domain.FlagOutput, domain.OutputJSON)
	r.out.next(t)
	r.out.next(t)
	repo, err := worktree.RepoOf(worktree.RepoOfParams{ProjectDir: dir})
	if err != nil {
		t.Fatal(err)
	}

	if err := process.Publish(process.PublishParams{SocketPath: process.SocketPath(), Repo: repo.CommonDir, Payload: json.RawMessage(`{"v":99,"type":"x","ts":"t"}`)}); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-r.done:
		if rules.ExitCode(err) != domain.ExitCodeEventsSchemaNewer {
			t.Fatalf("exit code %d (%v), want %d", rules.ExitCode(err), err, domain.ExitCodeEventsSchemaNewer)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the command kept going on a newer schema")
	}
}

// A consumer decides whether to retry from the exit code alone, so each final
// refusal is checked for its code, for a message, and for a stdout a JSON Lines
// reader can trust to be empty.
func TestAFinalRefusalExitsOnItsStableCodeAndWritesNothingOnStdout(t *testing.T) {
	cases := map[string]struct {
		cwd  func(t *testing.T) string
		args func(cwd string) []string
		code int
		says string
	}{
		"current directory outside git": {
			cwd:  func(t *testing.T) string { return t.TempDir() },
			args: func(string) []string { return nil },
			code: domain.ExitCodeNotGitRepo,
			says: domain.ErrNotGitRepo.Error(),
		},
		"--repo outside git": {
			cwd:  func(t *testing.T) string { return t.TempDir() },
			args: func(cwd string) []string { return []string{"--" + domain.FlagRepo, cwd} },
			code: domain.ExitCodeNotGitRepo,
			says: "--" + domain.FlagRepo,
		},
		"--repo not a directory": {
			cwd:  func(t *testing.T) string { return t.TempDir() },
			args: func(cwd string) []string { return []string{"--" + domain.FlagRepo, filepath.Join(cwd, "missing")} },
			code: domain.ExitCodeUsage,
			says: domain.FlagPathNotADirectory,
		},
		"repository not initialized": {
			cwd:  func(t *testing.T) string { return gittest.InitRepo(t) },
			args: func(string) []string { return nil },
			code: domain.ExitCodeConfigNotFound,
			says: "wtm init",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(domain.EnvProjectDir, "")
			t.Setenv(domain.EnvStateDir, "")
			cwd := c.cwd(t)
			t.Chdir(cwd)
			var stdout, stderr bytes.Buffer
			cmd := NewCmd()
			cmd.SetArgs(append(c.args(cwd), "--"+domain.FlagOutput, domain.OutputJSON))
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetContext(t.Context())
			// The root silences both and prints the error on stderr itself.
			cmd.SilenceUsage, cmd.SilenceErrors = true, true

			err := cmd.Execute()

			if got := rules.ExitCode(err); got != c.code {
				t.Fatalf("exit code %d (%v), want %d", got, err, c.code)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("err = %q, want it to say %q", err, c.says)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", stdout.String())
			}
		})
	}
}

type piped struct {
	read *os.File
	done chan error
}

func startPiped(t *testing.T) piped {
	t.Helper()
	processtest.Home(t)
	processtest.RealDaemon(t, process.SocketPath())
	dir := initializedRepo(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	cmd := NewCmd()
	cmd.SetArgs([]string{"--repo", dir, "--" + domain.FlagOutput, domain.OutputJSON})
	cmd.SetOut(w)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(t.Context())
	p := piped{read: r, done: make(chan error, 1)}
	go func() { p.done <- cmd.Execute() }()
	return p
}

func (p piped) endsCleanly(t *testing.T) {
	t.Helper()
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("a reader leaving is a success: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream outlived its reader")
	}
}

func TestTheStreamEndsWhenItsReaderLeavesAQuietRepository(t *testing.T) {
	p := startPiped(t)
	lines := bufio.NewReader(p.read)
	for range 2 {
		if _, err := lines.ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	}
	p.read.Close()
	p.endsCleanly(t)
}

func TestTheStreamEndsWhenItsReaderLeavesMidWrite(t *testing.T) {
	p := startPiped(t)
	if _, err := p.read.Read(make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	p.read.Close()
	p.endsCleanly(t)
}
