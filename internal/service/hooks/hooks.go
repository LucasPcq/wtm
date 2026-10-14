// Package hooks runs a worktree's lifecycle hooks.
package hooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/proc"
	"github.com/LucasPcq/wtm/internal/rules"
)

type RunHooksParams struct {
	Hooks   []domain.HookCommand
	WorkDir string
	Vars    rules.TemplateVars
	// Env is the vocabulary a run job gets: a hook that tears down docker needs the worktree's compose project as much as the job that brought it up.
	Env map[string]string
	// Output nil writes to os.Stderr.
	Output io.Writer
	// OnHook nil writes the beats to Output.
	OnHook func(domain.HookBeat)
}

// RunHooks stops on an interrupt whatever ContinueOnError says: the hook running is stopped, the rest never start.
func RunHooks(ctx context.Context, params RunHooksParams) error {
	output := params.Output
	if output == nil {
		output = os.Stderr
	}

	// os/exec copies Stdout and Stderr from two goroutines (distinct writer values are never deduped), and a sink need not be safe for concurrent use.
	sink := &syncWriter{w: output}

	report := params.OnHook
	if report == nil {
		report = writerReporter(sink)
	}

	for index, hook := range params.Hooks {
		if ctx.Err() != nil {
			return fmt.Errorf(domain.HooksInterruptedFmt, domain.ErrCancelled, len(params.Hooks)-index)
		}
		resolved := rules.ResolveTemplateVars(hook, params.Vars)
		err := runSingleHook(ctx, runSingleHookParams{
			Hook:       resolved,
			DefaultDir: params.WorkDir,
			Env:        params.Env,
			Output:     sink,
			Report:     report,
			// With no reporter installed, nobody has printed this hook's result
			// line and the error is the only thing the reader will see.
			Named: params.OnHook == nil,
		})
		if err == nil {
			continue
		}
		// An interrupt is not the hook's failure: continue_on_error does not go past it.
		if resolved.ContinueOnError && ctx.Err() == nil {
			continue
		}
		return err
	}

	return nil
}

type runSingleHookParams struct {
	Hook       domain.HookCommand
	DefaultDir string
	Env        map[string]string
	Output     io.Writer
	Report     func(domain.HookBeat)
	Named      bool
}

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

func writerReporter(w io.Writer) func(domain.HookBeat) {
	return func(beat domain.HookBeat) {
		if beat.Started {
			fmt.Fprintf(w, domain.HookFallbackStartFmt, rules.HookBeatLine(beat))
			return
		}
		fmt.Fprintf(w, domain.HookFallbackDoneFmt, rules.HookBeatLine(beat))
	}
}

func runSingleHook(ctx context.Context, params runSingleHookParams) error {
	hook := params.Hook

	if rules.IsBlankCommand(hook.Cmd) {
		return nil
	}

	spec := rules.ShellCommand(hook.Cmd)
	cmd := proc.GroupCommand(ctx, spec.Name, spec.Args...)

	if hook.Cwd != "" {
		cmd.Dir = hook.Cwd
	} else {
		cmd.Dir = params.DefaultDir
	}
	cmd.Env = hookEnv(params.Env)

	// stderr goes to the sink too: many tools put their progress there.
	var stderr bytes.Buffer
	cmd.Stdout = params.Output
	cmd.Stderr = io.MultiWriter(&stderr, params.Output)

	params.Report(domain.HookBeat{Cmd: hook.Cmd, Cwd: hook.Cwd, Started: true})

	start := time.Now()
	err := cmd.Run()

	beat := domain.HookBeat{Cmd: hook.Cmd, Cwd: hook.Cwd, Duration: time.Since(start)}
	if err != nil {
		beat.Err = err.Error()
		beat.Stderr = strings.TrimSpace(stderr.String())
	}
	params.Report(beat)

	if err != nil && ctx.Err() != nil {
		stopped := error(domain.ErrHookStopped)
		if params.Named {
			stopped = fmt.Errorf("%s: %w", hook.Cmd, domain.ErrHookStopped)
		}
		return failureOf(failureParams{Cmd: hook.Cmd, Wrapped: stopped})
	}
	if err != nil {
		wrapped := fmt.Errorf("%w: %w", domain.ErrHookFailed, err)
		if params.Named {
			wrapped = fmt.Errorf(domain.HookFailedNamedFmt, hook.Cmd, domain.ErrHookFailed, err)
		}
		return failureOf(failureParams{Cmd: hook.Cmd, Wrapped: wrapped, Cause: err})
	}
	return nil
}

func hookEnv(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return nil
	}
	// Every inherited worktree variable is cleared, so a name the worktree leaves unset is not filled by the launching shell's.
	return rules.MergeEnv(rules.MergeEnvParams{Env: os.Environ(), Clear: domain.WorktreeScopedEnv, Overrides: overrides})
}
