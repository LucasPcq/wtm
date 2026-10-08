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
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
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

	cmd := infra.GroupCommand(ctx, domain.ShellBin, domain.ShellCommandFlag, params.Command)
	cmd.Dir = params.Target.Path
	cmd.Env = params.Target.Env
	cmd.Stdout, cmd.Stderr = out, out

	begin := time.Now()
	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			result.Status = domain.ExecStatusInterrupted
			return result
		}
		result.Status = domain.ExecStatusFailed
		result.Error = err.Error()
		return result
	}
	waitErr := cmd.Wait()
	interrupted := ctx.Err() != nil

	result.DurationMs = time.Since(begin).Milliseconds()
	result.Tail = tail.Lines()
	if params.KeepOutput {
		result.Output = full.String()
	}
	if interrupted {
		result.Status = domain.ExecStatusInterrupted
		return result
	}
	code := exitCode(waitErr, cmd.ProcessState)
	result.ExitCode = &code
	result.Status = domain.ExecStatusPassed
	if code != 0 {
		result.Status = domain.ExecStatusFailed
	}
	return result
}

// exitCode reads the shell's own status when Wait only complained about the
// pipes it had to close: the command itself may well have passed.
func exitCode(err error, state *os.ProcessState) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, exec.ErrWaitDelay) && state != nil {
		return state.ExitCode()
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
