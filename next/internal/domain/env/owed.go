package env

import (
	"maps"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// NamespaceDrop returns how the namespace of job is dropped for the worktree
// whose job environment is jobEnv: its name, and the variables its remove
// command runs with, jobEnv's and the namespace's own; and whether there is
// one to drop: a namespace NamespaceFor would create, and a remove command.
func NamespaceDrop(job project.Job, jobEnv map[string]string) (NamespaceRun, bool, error) {
	namespace, made, err := NamespaceFor(job, jobEnv)
	if err != nil || !made || strings.TrimSpace(job.Namespace.Remove) == "" {
		return NamespaceRun{}, false, err
	}

	dropEnv := maps.Clone(jobEnv)
	maps.Copy(dropEnv, namespace.Env)
	return NamespaceRun{Name: namespace.Name, Env: dropEnv}, true, nil
}

// An OwedDrop is a namespace a removed worktree made in a shared service and
// still owes, as pending-removals.toml keeps it: the shared job, the
// worktree's slug and ordinal (all the legacy wtm keeps), and the namespace's
// name and the variables its remove command runs with (both empty in an
// entry the legacy wtm wrote).
type OwedDrop struct {
	Job      string
	Worktree string
	Ordinal  int
	Name     string
	Env      map[string]string
}

// Owe is the drop of job's namespace owed, made as NamespaceDrop gives it.
func Owe(job string, drop NamespaceRun) OwedDrop {
	ordinal, _ := strconv.Atoi(drop.Env[ordinalKey])
	return OwedDrop{Job: job, Worktree: drop.Env[WorktreeKey], Ordinal: ordinal, Name: drop.Name, Env: drop.Env}
}

// Drop returns how the drop owed is made of job's namespace: as it was kept,
// else, for an entry the legacy wtm wrote, from the worktree's slug and
// ordinal alone; and whether there is one to drop.
func (o OwedDrop) Drop(job project.Job) (NamespaceRun, bool, error) {
	if len(o.Env) == 0 {
		return NamespaceDrop(job, map[string]string{WorktreeKey: o.Worktree, ordinalKey: strconv.Itoa(o.Ordinal)})
	}
	if job.Namespace == nil || strings.TrimSpace(job.Namespace.Remove) == "" {
		return NamespaceRun{}, false, nil
	}
	return NamespaceRun{Name: o.Name, Env: o.Env}, true, nil
}
