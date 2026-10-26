package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Operation is the record of a mutating command in progress, as its holder file stores it and other processes read it.
type Operation struct {
	ID            string    `json:"op_id"`
	PID           int       `json:"pid"`
	Command       string    `json:"command"`
	Surface       Surface   `json:"surface"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	Locks         []Lock    `json:"locks"`
}

// Surface is where an operation was started from, as its record and its events name it: a closed set, published as an enum so that each surface words it.
type Surface string

// The surfaces an operation starts from.
const (
	SurfaceCLI Surface = "cli" // the command line
	SurfaceTUI Surface = "tui" // a command's wizard
	SurfaceWeb Surface = "web" // the web front wtm-next ui serves
	SurfaceAPI Surface = "api" // a call of the catalogue that names no surface
)

// Surfaces returns every Surface.
func Surfaces() []Surface {
	return []Surface{SurfaceCLI, SurfaceTUI, SurfaceWeb, SurfaceAPI}
}

// LockScope is what an operation lock covers: the whole repository, or one worktree.
type LockScope string

// The scopes of a lock.
const (
	ScopeRepo     LockScope = "repo"
	ScopeWorktree LockScope = "wt"
)

// LockMode says whether other operations may hold the same lock at once.
type LockMode string

// The modes of a lock.
const (
	LockShared    LockMode = "shared"
	LockExclusive LockMode = "exclusive"
)

// Lock is one lock an operation holds; Name is the worktree's for ScopeWorktree, empty for ScopeRepo.
type Lock struct {
	Scope LockScope `json:"scope"`
	Name  string    `json:"name,omitempty"`
	Mode  LockMode  `json:"mode"`
}

// Key returns the identity of the lock's resource, the same for its shared and exclusive modes: `repo`, or `wt:<name>`.
func (l Lock) Key() string {
	if l.Scope == ScopeWorktree {
		return string(l.Scope) + ":" + l.Name
	}

	return string(l.Scope)
}

// WorktreeLocks are the locks of an operation on worktrees: the repository shared, and each worktree named exclusive, once, sorted so that two operations always take their locks in the same order; an empty name is none.
func WorktreeLocks(worktrees []string) []Lock {
	locks := []Lock{{Scope: ScopeRepo, Mode: LockShared}}
	names := slices.DeleteFunc(slices.Clone(worktrees), func(name string) bool { return name == "" })
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		locks = append(locks, Lock{Scope: ScopeWorktree, Name: name, Mode: LockExclusive})
	}

	return locks
}

const correlationIDMaxBytes = 256

// CorrelationProblem returns what makes id unfit as a correlation ID (too long, or holding a control character), or nil when it is fit.
func CorrelationProblem(id string) error {
	if len(id) > correlationIDMaxBytes {
		return errors.New("is longer than 256 bytes")
	}
	if strings.IndexFunc(id, unicode.IsControl) >= 0 {
		return errors.New("contains a control character")
	}

	return nil
}

// CodePlanStale is the code of the refusal of a plan that no longer matches the fingerprint of its preview.
const CodePlanStale = "plan.stale"

// CodeReplan is the code of the follow-up of a stale plan: its dry run, run again on the facts as they are now.
const CodeReplan = "plan.replan"

// ReplanNote is the message of the follow-up of a stale plan.
const ReplanNote = "the plan the facts give now"

// Replan is the follow-up of err when it refuses a stale plan of command: its request, answers, run again as a dry run on the facts as they are now; none for any other err.
func Replan(err *Error, command string, answers []Answered) []FollowUp {
	if err == nil || err.Code != CodePlanStale {
		return nil
	}

	return []FollowUp{{Command: command, Answers: answers, DryRun: true, Code: CodeReplan, Message: ReplanNote}}
}

// RunOptions are how a surface runs an operation.
type RunOptions struct {
	DryRun            bool    // plan without taking locks, stop after the fingerprint
	ExpectFingerprint string  // refuse to apply a plan whose fingerprint differs; empty accepts any
	Surface           Surface // the surface that started the operation, recorded in its events
	CorrelationID     string  // ties the operation's events to the caller's own run; checked by [CorrelationProblem]
	Report            Report  // receives each event as it is published; may be nil
}

// PlanStale reports whether err is the refusal of a plan the facts changed under.
func PlanStale(err error) bool {
	found, ok := errors.AsType[*Error](err)
	return ok && found.Code == CodePlanStale
}
