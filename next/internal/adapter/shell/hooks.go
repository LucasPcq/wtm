package shell

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const (
	shellBin         = "/bin/sh"
	shellCommandFlag = "-c"
)

// RunHooks runs the hooks of params in order, each through /bin/sh -c, and
// stops at the first that fails unless it continues on error. It returns that
// failure as an *env.HookFailure; once ctx is cancelled, it stops the running
// hook and returns ctx's error with the count of hooks not run.
func RunHooks(ctx context.Context, params env.HookChain) error {
	sink := newLines(params.Output, params.Log)
	defer sink.close()

	for index, hook := range params.Hooks {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %d hook(s) not run", ctx.Err(), len(params.Hooks)-index)
		}

		err := runHook(ctx, hook, params, sink)
		sink.flush()
		if err == nil || hook.ContinueOnError && ctx.Err() == nil {
			continue
		}
		return err
	}
	return nil
}

// runHook runs one hook of chain, its output into sink. A blank command is
// skipped; a failure is an *env.HookFailure, with the exit code when the hook
// exited.
func runHook(ctx context.Context, hook project.Hook, chain env.HookChain, sink *lines) error {
	if strings.TrimSpace(hook.Cmd) == "" {
		return nil
	}

	sink.openLog()
	cmd := Command(ctx, shellBin, shellCommandFlag, hook.Cmd)
	cmd.Dir = hookDir(hook, chain.WorkDir)
	cmd.Env = hookEnv(chain)
	cmd.Stdout = &sink.stdout
	cmd.Stderr = &sink.stderr
	err := cmd.Run()
	if chain.Reap && cmd.Process != nil {
		reapGroup(cmd.Process.Pid)
	}
	switch {
	// A hook that exited 0 but left a child holding its output open still succeeded.
	case err == nil, errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState.Success():
		return nil
	}

	// the caller names the hook that failed ("on_clean hook failed"): the failure carries only its cause, a cancel's once cancelled
	failure := &env.HookFailure{Cmd: hook.Cmd, Err: err}
	if ctx.Err() != nil {
		failure.Err = ctx.Err()
	}
	if code, exited := ExitCode(err); exited {
		failure.ExitCode = &code
	}
	return failure
}

// hookDir returns where hook runs: workDir, or its cwd resolved against it.
func hookDir(hook project.Hook, workDir string) string {
	if hook.Cwd == "" {
		return workDir
	}
	if filepath.IsAbs(hook.Cwd) {
		return hook.Cwd
	}
	return filepath.Join(workDir, hook.Cwd)
}

// hookEnv returns the environment of the chain's hooks, nil (this process's
// own) when the chain neither sets nor unsets any.
func hookEnv(params env.HookChain) []string {
	if len(params.Env) == 0 && len(params.Unset) == 0 {
		return nil
	}
	return Environ(params.Unset, params.Env)
}
