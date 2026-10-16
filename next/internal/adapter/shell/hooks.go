package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/hookvars"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

const (
	shellBin          = "/bin/sh"
	shellCommandFlag  = "-c"
	signalledExitCode = -1
)

var (
	ErrHookFailed  = errors.New("hook failed")
	ErrHookStopped = errors.New("hook stopped")

	worktreeScopedEnv = []string{"WTM_WORKTREE", "WTM_BRANCH", "WTM_ORDINAL", "WTM_PORT_OFFSET", "COMPOSE_PROJECT_NAME", "WTM_ISOLATION"}
)

type Failure struct {
	Cmd      string
	ExitCode *int
	Err      error
}

func (f *Failure) Error() string { return f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }

type HooksParams struct {
	Hooks   []project.Hook
	WorkDir string
	Env     map[string]string
	Vars    hookvars.Vars
	Output  func(line string)
	Log     string
}

func RunHooks(ctx context.Context, params HooksParams) error {
	sink := newLines(linesParams{Emit: params.Output, Log: params.Log})
	defer sink.close()
	for index, hook := range params.Hooks {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %d hook(s) not run", ctx.Err(), len(params.Hooks)-index)
		}
		resolved := params.Vars.Hook(hook)
		err := runHook(ctx, hookRun{Hook: resolved, Params: params, Sink: sink})
		sink.flush()
		if err == nil || resolved.ContinueOnError && ctx.Err() == nil {
			continue
		}
		return err
	}
	return nil
}

type hookRun struct {
	Hook   project.Hook
	Params HooksParams
	Sink   *lines
}

func runHook(ctx context.Context, run hookRun) error {
	if strings.TrimSpace(run.Hook.Cmd) == "" {
		return nil
	}
	run.Sink.openLog()
	cmd := Command(ctx, shellBin, shellCommandFlag, run.Hook.Cmd)
	cmd.Dir = hookDir(run)
	cmd.Env = hookEnv(run.Params.Env)
	cmd.Stdout = run.Sink
	cmd.Stderr = run.Sink
	err := cmd.Run()
	switch {
	case err == nil, errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState.Success():
		return nil
	case ctx.Err() != nil:
		return &Failure{Cmd: run.Hook.Cmd, Err: fmt.Errorf("%w: %w", ErrHookStopped, ctx.Err())}
	}
	return &Failure{Cmd: run.Hook.Cmd, ExitCode: exitCodeOf(err), Err: fmt.Errorf("%w: %w", ErrHookFailed, err)}
}

func hookDir(run hookRun) string {
	if run.Hook.Cwd == "" {
		return run.Params.WorkDir
	}
	if filepath.IsAbs(run.Hook.Cwd) {
		return run.Hook.Cwd
	}
	return filepath.Join(run.Params.WorkDir, run.Hook.Cwd)
}

func exitCodeOf(err error) *int {
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() <= signalledExitCode {
		return nil
	}
	code := exit.ExitCode()
	return &code
}

func hookEnv(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return nil
	}
	env := slices.DeleteFunc(os.Environ(), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		_, overridden := overrides[key]
		return overridden || slices.Contains(worktreeScopedEnv, key)
	})
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		env = append(env, key+"="+overrides[key])
	}
	return env
}
