package events

import (
	"cmp"
	"maps"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The facts

// Facts are what a snapshot reads of a repository: its worktrees, and the meta record of each worktree on a branch that has one, by branch.
type Facts struct {
	worktree.Repository
	Metas map[string]worktree.Meta
}

// Identities are the worktrees of facts as the stream names them, those on no branch left out: a reader keys a worktree by its branch.
// A worktree's ordinal is its record's while no other worktree whose branch sorts first claims the same, the main worktree holding 0; its isolation is its record's, isolated without one.
func Identities(facts Facts) []worktree.Identity {
	identities := make([]worktree.Identity, 0, len(facts.Worktrees))
	for _, w := range facts.Worktrees {
		branch := w.Branch()
		if branch == "" {
			continue
		}

		meta := facts.Metas[branch]
		identities = append(identities, worktree.Identity{
			Branch:    branch,
			Path:      w.Path,
			Parent:    meta.SourceBranch,
			Ordinal:   facts.ordinal(w),
			Isolation: meta.Isolation.Effective(),
			Main:      w.Main,
			CreatedAt: meta.CreatedAt,
		})
	}

	return identities
}

// ordinal is the number w holds: 0 for the main worktree, its record's when no other worktree whose branch sorts first claims it, else none yet.
func (f Facts) ordinal(w worktree.Worktree) *int {
	if w.Main {
		return new(0)
	}

	claimed := f.Metas[w.Branch()].Ordinal
	if claimed <= 0 {
		return nil
	}
	for _, other := range f.Worktrees {
		branch := other.Branch()
		if branch == "" || branch == w.Branch() {
			continue
		}
		if f.claim(other) == claimed && branch < w.Branch() {
			return nil
		}
	}

	return new(claimed)
}

// claim is the ordinal w claims: 0 for the main worktree, else its record's.
func (f Facts) claim(w worktree.Worktree) int {
	if w.Main {
		return 0
	}
	return f.Metas[w.Branch()].Ordinal
}

// ----------------------------------------------------------------------------
// The poll

// A Polled is what a poll reads of a repository: each worktree on a branch, by its branch.
type Polled map[string]Poll

// A Poll is what a poll reads of one worktree: its branch's tip, what it has checked out, each of its jobs with its state, sorted, and whether it holds uncommitted changes; Jobs is nil when the run daemon could not tell, Dirty when git could not.
type Poll struct {
	Head     string
	Checkout string
	Jobs     []string
	Dirty    *bool
}

// checkoutBranch is the checkout of a worktree on its branch with no operation under way; any other is the operation git stopped in the middle of.
const checkoutBranch = "branch"

// PollOf is what a poll reads of the worktrees and branches of snapshot, the jobs of the machine being jobs, or unknown when known is false, and whether each worktree is dirty by its path, unknown for a path dirty lacks.
func PollOf(snapshot worktree.Snapshot, jobs []run.Job, known bool, dirty map[string]bool) Polled {
	tips := map[string]string{}
	for _, branch := range snapshot.Branches.Local {
		tips[branch.Name] = branch.Tip
	}

	polled := Polled{}
	for _, w := range snapshot.Worktrees {
		branch := w.Branch()
		if branch == "" {
			continue
		}
		poll := Poll{Head: tips[branch], Checkout: cmp.Or(string(w.Operation()), checkoutBranch)}
		if known {
			poll.Jobs = jobsIn(jobs, w.Path)
		}
		if d, read := dirty[w.Path]; read {
			poll.Dirty = new(d)
		}
		polled[branch] = poll
	}
	return polled
}

// jobsIn is each job of the worktree at dir with its state, sorted, empty and not nil when it has none.
func jobsIn(jobs []run.Job, dir string) []string {
	in := []string{}
	for _, job := range jobs {
		if job.WorkDir == dir {
			in = append(in, job.Name+"="+string(job.State))
		}
	}
	slices.Sort(in)
	return in
}

// A Change is what a poll found moved in the worktree of Branch.
type Change struct {
	Branch string
	Fields []worktree.PolledField
}

// Changes returns what changed from before to now: each worktree whose head, checkout, jobs or dirtiness changed, by branch, a worktree new to the poll having every field changed, and the branches of the worktrees that went away, for which a reader resets from a fresh snapshot. Jobs or dirtiness unknown before or now are not compared; they are taken as they were.
func Changes(before, now Polled) (changes []Change, gone []string) {
	for _, branch := range slices.Sorted(maps.Keys(before)) {
		if _, kept := now[branch]; !kept {
			gone = append(gone, branch)
		}
	}

	for _, branch := range slices.Sorted(maps.Keys(now)) {
		fields := []worktree.PolledField{worktree.PolledHead, worktree.PolledCheckout, worktree.PolledJobs, worktree.PolledDirty}
		if was, seen := before[branch]; seen {
			fields = polledFields(was, now[branch])
		}
		if len(fields) > 0 {
			changes = append(changes, Change{Branch: branch, Fields: fields})
		}
	}
	return changes, gone
}

func polledFields(was, now Poll) []worktree.PolledField {
	var fields []worktree.PolledField
	if was.Head != now.Head {
		fields = append(fields, worktree.PolledHead)
	}
	if was.Checkout != now.Checkout {
		fields = append(fields, worktree.PolledCheckout)
	}
	if was.Jobs != nil && now.Jobs != nil && !slices.Equal(was.Jobs, now.Jobs) {
		fields = append(fields, worktree.PolledJobs)
	}
	if was.Dirty != nil && now.Dirty != nil && *was.Dirty != *now.Dirty {
		fields = append(fields, worktree.PolledDirty)
	}
	return fields
}

// Carried is now with the jobs and the dirtiness of each worktree unknown now taken as before knew them, so that a poll the run daemon or git could not answer changes nothing.
func Carried(before, now Polled) Polled {
	for branch, poll := range now {
		if poll.Jobs == nil {
			poll.Jobs = before[branch].Jobs
		}
		if poll.Dirty == nil {
			poll.Dirty = before[branch].Dirty
		}
		now[branch] = poll
	}
	return now
}

// ----------------------------------------------------------------------------
// Reconnecting

const (
	backoffFirst = 200 * time.Millisecond
	backoffLimit = 5 * time.Second
)

// A Backoff is the wait before each attempt to subscribe again: the first one after an attempt that reached ready, then twice the last, up to a limit.
type Backoff struct {
	last time.Duration
}

// Next returns the wait before the next attempt, the last one having reached ready or not.
func (b *Backoff) Next(reachedReady bool) time.Duration {
	if reachedReady {
		b.last = 0
	}
	b.last = cmp.Or(min(b.last*2, backoffLimit), backoffFirst)
	return b.last
}
