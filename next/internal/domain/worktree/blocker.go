package worktree

// A Blocker is what makes a worktree unsafe to remove: a safety refusal, which only forcing lifts.
type Blocker string

// The blockers of a removal, in the order a person acts on them.
const (
	BlockerLocked          Blocker = "locked"            // git worktree lock: someone set it on purpose
	BlockerDirty           Blocker = "dirty"             // uncommitted changes
	BlockerUnpushed        Blocker = "unpushed"          // commits no other branch nor remote holds
	BlockerOpenPullRequest Blocker = "open_pull_request" // a pull request still open
	BlockerJobsRunning     Blocker = "jobs_running"      // jobs of the run daemon up in it
)

// Blockers returns every Blocker.
func Blockers() []Blocker {
	return []Blocker{BlockerLocked, BlockerDirty, BlockerUnpushed, BlockerOpenPullRequest, BlockerJobsRunning}
}
