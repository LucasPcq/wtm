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

func runHook(ctx context.Context, hook project.Hook, chain env.HookChain, sink *lines) error {
	if strings.TrimSpace(hook.Cmd) == "" {
		return nil
	}
	sink.openLog()
	cmd := Command(ctx, shellBin, shellCommandFlag, hook.Cmd)
	cmd.Dir = hookDir(hook, chain.WorkDir)
	cmd.Env = hookEnv(chain)
	cmd.Stdout = sink
	cmd.Stderr = sink
	err := cmd.Run()
	switch {
	case err == nil, errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState.Success():
		return nil
	case ctx.Err() != nil:
		return &env.HookFailure{Cmd: hook.Cmd, Err: fmt.Errorf("hook stopped: %w", ctx.Err())}
	}
	failure := &env.HookFailure{Cmd: hook.Cmd, Err: fmt.Errorf("hook failed: %w", err)}
	if code, exited := ExitCode(err); exited {
		failure.ExitCode = &code
	}
	return failure
}

func hookDir(hook project.Hook, workDir string) string {
	if hook.Cwd == "" {
		return workDir
	}
	if filepath.IsAbs(hook.Cwd) {
		return hook.Cwd
	}
	return filepath.Join(workDir, hook.Cwd)
}

func hookEnv(params env.HookChain) []string {
	if len(params.Env) == 0 {
		return nil
	}
	return Environ(params.Unset, params.Env)
}
