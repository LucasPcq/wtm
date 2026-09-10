// Package hooks implements the execution engine for on_create hooks.
package hooks

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// RunHooksParams holds inputs for executing lifecycle hooks.
type RunHooksParams struct {
	Hooks   []domain.HookCommand
	WorkDir string
	Vars    rules.TemplateVars
	// Env is what the hook learns about the worktree it runs in, the same
	// vocabulary a run job gets. A hook that tears down docker needs the
	// worktree's compose project as much as the job that brought it up.
	Env    map[string]string
	Output io.Writer // if nil, uses os.Stdout/Stderr (CLI mode). Set to capture output (TUI mode).
	// OnHook receives each hook starting and finishing. A surface that reports
	// those beats itself sets it; nil falls back to writing them to Output, which
	// is the only rendering this package does and only because someone must.
	OnHook func(domain.HookBeat)
}

// RunHooks executes each hook command sequentially with template interpolation.
// Stops on first error unless the hook has ContinueOnError set.
func RunHooks(params RunHooksParams) error {
	output := params.Output
	if output == nil {
		output = os.Stderr
	}

	// os/exec gives a hook two copier goroutines — Stdout and Stderr are distinct
	// writer values, so it never dedupes them — and a sink is not required to be
	// safe for concurrent use. Serializing here means every surface gets that for
	// free, rather than each having to defend itself.
	sink := &syncWriter{w: output}

	report := params.OnHook
	if report == nil {
		report = writerReporter(sink)
	}

	for _, hook := range params.Hooks {
		resolved := rules.ResolveTemplateVars(hook, params.Vars)
		err := runSingleHook(runSingleHookParams{
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
		if resolved.ContinueOnError {
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

// writerReporter is the fallback rendering for a caller that gave no reporter:
// three visually separated beats — the command, its live output, the result —
// so a streamed install is readable with nobody drawing anything. It is the only
// formatting this package does, and only because someone has to when a caller
// declines to.
func writerReporter(w io.Writer) func(domain.HookBeat) {
	return func(beat domain.HookBeat) {
		if beat.Started {
			fmt.Fprintf(w, domain.HookFallbackStartFmt, rules.HookBeatLine(beat))
			return
		}
		fmt.Fprintf(w, domain.HookFallbackDoneFmt, rules.HookBeatLine(beat))
	}
}

func runSingleHook(params runSingleHookParams) error {
	hook := params.Hook

	if rules.IsBlankCommand(hook.Cmd) {
		return nil
	}

	spec := rules.ShellCommand(hook.Cmd)
	cmd := exec.Command(spec.Name, spec.Args...)

	if hook.Cwd != "" {
		cmd.Dir = hook.Cwd
	} else {
		cmd.Dir = params.DefaultDir
	}
	cmd.Env = hookEnv(params.Env)

	// stderr goes to the sink as well as into the buffer: a great many tools put
	// their progress there, and a surface showing only stdout would sit silent
	// for the whole hook and keep a log that is not the whole output.
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

	if err != nil {
		if params.Named {
			return fmt.Errorf(domain.HookFailedNamedFmt, hook.Cmd, domain.ErrHookFailed, err)
		}
		return fmt.Errorf("%w: %w", domain.ErrHookFailed, err)
	}
	return nil
}

// hookEnv layers the worktree's variables over this process's environment. Nil
// leaves it as it is: unlike the run daemon, this process is the user's own
// command, so what it inherited is the user's and not another worktree's.
func hookEnv(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return nil
	}
	return rules.MergeEnv(rules.MergeEnvParams{Env: os.Environ(), Overrides: overrides})
}
