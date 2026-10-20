package cli

import (
	"os"
	"path/filepath"
)

// The environment variables a command line run reads.
const (
	projectDirEnv    = "WTM_PROJECT_DIR"    // names the repository instead of the working directory's
	CorrelationIDEnv = "WTM_CORRELATION_ID" // ties an operation's events to the caller's own run
)

// An Env is what a command line run takes from its process, read once by [ReadEnv]: the directory it runs in, the one it opens the repository from, and the correlation ID [Run.Begin] checks.
type Env struct {
	Cwd           string // the working directory with its symbolic links resolved, from which paths are shown
	ProjectDir    string // $WTM_PROJECT_DIR with its symbolic links resolved, else the working directory as the system gives it
	Named         bool   // whether $WTM_PROJECT_DIR named ProjectDir
	CorrelationID string // $WTM_CORRELATION_ID
}

// ReadEnv reads the process's Env. A working directory that cannot be read leaves Cwd empty: paths then show absolute, and opening the repository from it fails on its own.
func ReadEnv() Env {
	cwd, _ := os.Getwd()
	env := Env{Cwd: resolved(cwd), ProjectDir: cwd, CorrelationID: os.Getenv(CorrelationIDEnv)}
	if named := os.Getenv(projectDirEnv); named != "" {
		env.ProjectDir, env.Named = resolved(named), true
	}

	return env
}

// resolved returns path with its symbolic links resolved, or as it is when they cannot be; "" stays "".
func resolved(path string) string {
	if path == "" {
		return ""
	}
	if resolvedPath, err := filepath.EvalSymlinks(path); err == nil {
		return resolvedPath
	}
	return path
}
