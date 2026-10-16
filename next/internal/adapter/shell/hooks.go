package shell

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/hook"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

const (
	shellBin         = "/bin/sh"
	shellCommandFlag = "-c"
)

func RunHooks(ctx context.Context, params hook.Chain) error {
	sink := newLines(linesParams{Emit: params.Output, Log: params.Log})
	defer sink.close()
	for index, hook := range params.Hooks {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %d hook(s) not run", ctx.Err(), len(params.Hooks)-index)
		}
		err := runHook(ctx, hookRun{Hook: hook, Params: params, Sink: sink})
		sink.flush()
		if err == nil || hook.ContinueOnError && ctx.Err() == nil {
			continue
		}
		return err
	}
	return nil
}

type hookRun struct {
	Hook   project.Hook
	Params hook.Chain
	Sink   *lines
}

func runHook(ctx context.Context, run hookRun) error {
	if strings.TrimSpace(run.Hook.Cmd) == "" {
		return nil
	}
	run.Sink.openLog()
	cmd := Command(ctx, shellBin, shellCommandFlag, run.Hook.Cmd)
	cmd.Dir = hookDir(run)
	cmd.Env = hookEnv(run.Params)
	cmd.Stdout = run.Sink
	cmd.Stderr = run.Sink
	err := cmd.Run()
	switch {
	case err == nil, errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState.Success():
		return nil
	case ctx.Err() != nil:
		return &hook.Failure{Cmd: run.Hook.Cmd, Err: fmt.Errorf("hook stopped: %w", ctx.Err())}
	}
	failure := &hook.Failure{Cmd: run.Hook.Cmd, Err: fmt.Errorf("hook failed: %w", err)}
	if code, exited := ExitCode(err); exited {
		failure.ExitCode = &code
	}
	return failure
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

func hookEnv(params hook.Chain) []string {
	if len(params.Env) == 0 {
		return nil
	}
	return Environ(EnvironParams{Unset: params.Unset, Set: params.Env})
}
