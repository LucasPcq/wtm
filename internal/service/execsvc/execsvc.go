package execsvc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type Target struct {
	Branch  string
	Path    string
	Env     []string
	LogPath string
}

type RunParams struct {
	Command    string
	Targets    []Target
	Jobs       int
	KeepOutput bool
	OnBeat     func(domain.ExecBeat)
}

func Run(ctx context.Context, params RunParams) []domain.ExecResult {
	results := make([]domain.ExecResult, len(params.Targets))
	for i, target := range params.Targets {
		results[i] = domain.ExecResult{Branch: target.Branch, Path: target.Path, Status: domain.ExecStatusNotStarted}
	}

	var beatMu sync.Mutex
	beat := func(b domain.ExecBeat) {
		if params.OnBeat == nil {
			return
		}
		beatMu.Lock()
		defer beatMu.Unlock()
		params.OnBeat(b)
	}

	slots := make(chan struct{}, max(params.Jobs, 1))
	var wg sync.WaitGroup
	for i, target := range params.Targets {
		select {
		case <-ctx.Done():
			wg.Wait()
			return results
		case slots <- struct{}{}:
		}
		if ctx.Err() != nil {
			<-slots
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			beat(domain.ExecBeat{Index: i, Started: true, Result: results[i]})
			results[i] = runOne(ctx, runOneParams{Command: params.Command, Target: target, KeepOutput: params.KeepOutput})
			beat(domain.ExecBeat{Index: i, Result: results[i]})
		}()
	}
	wg.Wait()
	return results
}

type runOneParams struct {
	Command    string
	Target     Target
	KeepOutput bool
}

func runOne(ctx context.Context, params runOneParams) domain.ExecResult {
	result := domain.ExecResult{Branch: params.Target.Branch, Path: params.Target.Path, Log: params.Target.LogPath}
	tail := newTail(domain.ExecJSONTailLines)
	var full bytes.Buffer
	sinks := []io.Writer{tail}
	if params.KeepOutput {
		sinks = append(sinks, &full)
	}
	if log := openLog(params.Target.LogPath); log != nil {
		defer func() { _ = log.Close() }()
		sinks = append(sinks, log)
	} else {
		result.Log = ""
	}
	out := &lockedWriter{w: io.MultiWriter(sinks...)}

	cmd := exec.Command(domain.ShellBin, domain.ShellCommandFlag, params.Command)
	cmd.Dir = params.Target.Path
	cmd.Env = params.Target.Env
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	begin := time.Now()
	if err := cmd.Start(); err != nil {
		result.Status = domain.ExecStatusFailed
		result.Error = err.Error()
		return result
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	interrupted := false
	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		interrupted = true
		waitErr = stop(stopParams{Pid: cmd.Process.Pid, Done: done})
	}

	result.DurationMs = time.Since(begin).Milliseconds()
	result.Tail = tail.Lines()
	if params.KeepOutput {
		result.Output = full.String()
	}
	if interrupted {
		result.Status = domain.ExecStatusInterrupted
		return result
	}
	code := exitCode(waitErr)
	result.ExitCode = &code
	result.Status = domain.ExecStatusPassed
	if code != 0 {
		result.Status = domain.ExecStatusFailed
	}
	return result
}

type stopParams struct {
	Pid  int
	Done <-chan error
}

// stop gives the group SIGINT and a grace period, then SIGKILL: a command that
// traps SIGINT must not hang the whole run.
func stop(params stopParams) error {
	_ = syscall.Kill(-params.Pid, syscall.SIGINT)
	select {
	case err := <-params.Done:
		return err
	case <-time.After(domain.ExecInterruptGrace):
		_ = syscall.Kill(-params.Pid, syscall.SIGKILL)
		return <-params.Done
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func openLog(path string) *os.File {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil
	}
	return file
}

// lockedWriter: os/exec copies stdout and stderr on two goroutines.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
