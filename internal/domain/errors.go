package domain

import "errors"

var (
	// ErrWorktreeNotFound is returned when a worktree cannot be located.
	ErrWorktreeNotFound = errors.New("worktree not found")

	// ErrWorktreeExists is returned when attempting to create a duplicate worktree.
	ErrWorktreeExists = errors.New("worktree already exists")

	// ErrConfigNotFound is returned when no configuration file is found.
	ErrConfigNotFound = errors.New("config file not found")

	// ErrInvalidConfig is returned when configuration fails validation.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrInvalidEnvStrategy is returned when env.strategy has an unknown value.
	ErrInvalidEnvStrategy = errors.New("invalid env strategy: must be example, main, or parent")

	// ErrInvalidShellType is returned when shell has an unknown value.
	ErrInvalidShellType = errors.New("invalid shell type: must be zsh, bash, or fish")

	// ErrUserAborted is returned when the user cancels an interactive prompt.
	ErrUserAborted = errors.New("user aborted")

	// ErrNamespaceUnknownToken names the placeholder a namespace value used and wtm
	// does not define. Caught at load rather than in a shell, where literal
	// braces would silently create a database called "{branch}".
	ErrNamespaceUnknownToken = errors.New("unknown placeholder in a [job.namespace] value")
	// ErrNoMainCheckout is a repository with no main worktree to run a shared
	// job in — a bare clone whose worktrees are all linked.
	ErrNoMainCheckout = errors.New("no main checkout to run a shared job in")

	// ErrAborted signals a command that failed after already printing its own
	// report (e.g. a profile aborted by a failing task). The top-level handler
	// exits non-zero without printing a second, redundant error line.
	ErrAborted = errors.New("aborted")

	// ErrNotGitRepo is returned when the current directory is not a git repository.
	ErrNotGitRepo = errors.New("not a git repository")

	// ErrBranchNotFound is returned when a specified source or parent branch does
	// not exist as a local branch or an origin remote-tracking branch.
	ErrBranchNotFound = errors.New("branch not found")

	// ErrWorktreePathExists is returned when the target worktree directory already exists.
	ErrWorktreePathExists = errors.New("worktree path already exists")

	// ErrInvalidBasePath is returned when --to is not a usable repo-relative path
	// (blank/whitespace-only or absolute).
	ErrInvalidBasePath = errors.New("invalid base path: must be a non-empty path relative to the repo root")

	// ErrCannotCleanParent is returned when trying to clean the parent worktree.
	ErrCannotCleanParent = errors.New("cannot clean the parent worktree")

	// ErrWorktreeRemoveFailed wraps a failure of the `git worktree remove` step in
	// Clean, so the command layer can offer the interactive `sudo rm -rf` fallback
	// (files owned by root — e.g. Docker — that git cannot delete as the current
	// user) instead of aborting.
	ErrWorktreeRemoveFailed = errors.New("worktree removal failed")

	// ErrOrdinalRefIncomplete is returned when an ordinal is asked for without
	// naming the worktree it belongs to. Allocation writes to the state dir and
	// keys on the branch, so an empty one would resolve relative to the current
	// directory instead of failing.
	ErrOrdinalRefIncomplete = errors.New("worktree reference incomplete: project dir, state dir and branch are all required")

	// ErrOrdinalUnreadable is returned when a live worktree's metadata exists but
	// cannot be read, so what it holds is unknown. Treating that as "holds
	// nothing" would hand its number to another worktree, which is the collision
	// the ordinal exists to prevent.
	ErrOrdinalUnreadable = errors.New("cannot read a live worktree's ordinal")

	// ErrGHNotInstalled is returned when the gh CLI is not found on PATH.
	ErrGHNotInstalled = errors.New("gh CLI not found — install it from https://cli.github.com")

	// ErrGHNotAuthenticated is returned when gh is not logged in to GitHub.
	ErrGHNotAuthenticated = errors.New("not logged in to GitHub — run 'gh auth login'")

	// ErrJobNotFound is returned when a referenced job is not declared in run.toml.
	ErrJobNotFound = errors.New("job not found")

	// ErrJobNotAttachable is returned for a job with no live output to subscribe
	// to: a detached launcher, whose stream ended with the launcher, or a job
	// that is no longer running. Its log file is what is left to read.
	ErrJobNotAttachable = errors.New("job has no live output")

	// ErrRunServiceRequired is returned by the run log seam when it was handed no
	// daemon to talk to — a wiring mistake in the surface, never a user error.
	ErrRunServiceRequired = errors.New("run log service is required")

	// ErrJobStreamClosed is returned by a subscription that has been closed. Close
	// is a barrier: what the surface asks of the job afterwards is refused rather
	// than sent on its behalf.
	ErrJobStreamClosed = errors.New("job stream is closed")

	// ErrRunNotInitialized is returned when a run command runs before the run
	// module is initialized — run.toml is absent or declares no job/profile. The
	// message points at the dedicated setup command.
	ErrRunNotInitialized = errors.New("run module not initialized — run `wtm run init` first")

	// ErrExtractConflict is returned when the selected changes do not apply
	// cleanly onto the target worktree. The extraction is aborted and the source
	// worktree is left untouched.
	ErrExtractConflict = errors.New("cannot apply the selected changes")

	// ErrPorcelainMalformed is returned when a `git status --porcelain -z` record
	// is truncated: shorter than the status field plus a path, or a rename record
	// with no origin-path field behind it.
	ErrPorcelainMalformed = errors.New("malformed git status record")

	// ErrNoChangesToExtract is returned when the source worktree has no
	// uncommitted changes to extract.
	ErrNoChangesToExtract = errors.New("no uncommitted changes to extract")

	// ErrNoDirtyWorktrees is returned when no managed worktree has uncommitted
	// changes, so the interactive source picker would be empty.
	ErrNoDirtyWorktrees = errors.New("no worktree has changes to extract")

	// ErrExtractSourceRequired is returned when extract is invoked without a
	// source worktree argument and cannot fall back to the interactive picker
	// (--yes, no terminal, or --output json).
	ErrExtractSourceRequired = errors.New("specify a source worktree (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrExtractFilesRequired is returned when extract cannot resolve which files
	// to extract: --files was omitted and there is no interactive picker (--yes, no
	// terminal, or --output json).
	ErrExtractFilesRequired = errors.New("specify --files (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrExtractTargetRequired is returned when extract cannot resolve the target
	// worktree: --to was omitted and there is no interactive picker (--yes, no
	// terminal, or --output json).
	ErrExtractTargetRequired = errors.New("specify --to (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrCleanBranchRequired is returned when clean is invoked without a worktree
	// branch argument and cannot fall back to the interactive picker (--yes, no
	// terminal, or --output json).
	ErrCleanBranchRequired = errors.New("specify a worktree branch to clean (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrNoFilesSelected is returned when the user selects no files to extract.
	ErrNoFilesSelected = errors.New("no files selected")

	// ErrSameWorktree is returned when the target worktree equals the source.
	ErrSameWorktree = errors.New("target worktree must differ from the source")

	// ErrReparentSelf is returned when a worktree is asked to become its own parent.
	ErrReparentSelf = errors.New("a worktree cannot be its own parent")

	// ErrReparentBranchesRequired and ErrReparentParentRequired are the two
	// selections reparent cannot default: guessing either would rewrite a stack
	// the user did not ask for.
	ErrReparentBranchesRequired = errors.New("specify at least one worktree (no interactive picker under --yes, without a terminal, or in --output json mode)")
	ErrReparentParentRequired   = errors.New("specify the new parent with --to (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrEnvWorktreeRequired is returned when `wtm env` is invoked without a worktree
	// argument and cannot fall back to the interactive picker (--yes, no terminal, or
	// --output json).
	ErrEnvWorktreeRequired = errors.New("specify a worktree (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrEnvJSONNeedsYes is returned when `wtm env` runs in --output json without
	// --yes: interactive resolution cannot run in JSON mode.
	ErrEnvJSONNeedsYes = errors.New("--output json requires --yes (interactive resolution cannot run in JSON mode)")

	// ErrEnvNoFiles is returned when `wtm env` runs on a project whose config
	// declares no env files to reconcile.
	ErrEnvNoFiles = errors.New("no env files configured — run `wtm init --only env` to detect them")

	// ErrEnvFileNoTarget is returned when an env.file entry has an empty target.
	ErrEnvFileNoTarget = errors.New("env file entry must have a target")

	// ErrEnvFileDuplicateTarget is returned when two env.file entries share a target.
	ErrEnvFileDuplicateTarget = errors.New("duplicate env file target")

	// ErrEnvFileBadTemplate is returned when an env.file template is not a known
	// template of its target (a recognized suffix appended to the target path).
	ErrEnvFileBadTemplate = errors.New("env file template must be a known template of its target")

	// ErrCleanJSONNeedsYes is returned when clean runs in --output json without
	// --yes: confirmations cannot run in JSON mode.
	ErrCleanJSONNeedsYes = errors.New("--output json requires --yes (confirmations cannot run in JSON mode; add --force to lift safety checks)")

	// ErrUnsafeSudoDeletePath is returned when the `sudo rm -rf` recovery path
	// would target an obviously dangerous location (a filesystem root, the home
	// directory, the repository root, or an ancestor of it) — a defensive guard
	// against corrupted git worktree metadata before privilege escalation.
	ErrUnsafeSudoDeletePath = errors.New("refusing to sudo rm -rf an unsafe path")

	// ErrDashboardNotInteractive is returned when `wtm ui` is invoked without a
	// terminal on both ends: a full-screen dashboard has no non-interactive form.
	ErrDashboardNotInteractive = errors.New("`wtm ui` needs a terminal — the dashboard cannot be driven by an agent; use `wtm list --output json`")

	// ErrJobAmbiguous is returned when several jobs publish a URL and the caller
	// named none — a picker needs a fully interactive run, so the flag is the answer.
	ErrJobAmbiguous = errors.New("several jobs publish a URL: name one")

	// ErrJobRequired is returned when `run start` / `run stop` cannot resolve
	// which job to act on: a required selection with no safe default, so it names
	// the flag rather than falling back to a picker.
	ErrJobRequired = errors.New("specify --job (no interactive picker without a terminal or in --output json mode)")

	// ErrExclusiveMultiWorktree refuses a flag that contradicts itself: exclusive
	// means one stack at a time, and the run was told to bring up several. Unlike
	// the setting, a flag leaves nothing to put to anyone — it was typed for this
	// run, against what the same command line asked for.
	ErrExclusiveMultiWorktree = errors.New("--exclusive cannot start several worktrees: it stops all but one")

	// ErrNoJobsDeclared is a job step with nothing to offer: the question cannot
	// be asked, and the answer is to declare a job rather than to pick one.
	ErrNoJobsDeclared = errors.New("no jobs declared in run.toml")

	// ErrNoProfilesDeclared is ErrNoJobsDeclared's counterpart for the profile
	// axis: a picker with nothing in it is not a question.
	ErrNoProfilesDeclared = errors.New("no profiles declared in run.toml")

	// ErrProxyNoListeners means launchd started the forwarder without handing it
	// the sockets it exists to serve.
	ErrProxyNoListeners = errors.New("launchd started the port-80 forwarder with no listening socket")

	// ErrProxyNoTarget is a forwarder built without a way to find the proxy.
	ErrProxyNoTarget = errors.New("the port-80 forwarder was given no way to resolve the run proxy's port")

	// ErrProxyInstallNeedsYes is the ordinary confirmation axis: the write itself
	// needs no privilege, so a run nobody can answer only needs --yes.
	ErrProxyInstallNeedsYes = errors.New("`wtm run proxy install` installs a LaunchAgent — pass --yes to run it without a terminal")

	// ErrProxyRedirectUnsupported is what every platform but darwin answers: the
	// named URLs keep their port there, which is a state, not a failure.
	ErrProxyRedirectUnsupported = errors.New("serving the proxy on port 80 is not implemented on this platform yet — named URLs keep their port")

	// ErrJobNonePublished is returned when no job in run.toml declares a url, so
	// there is no address to print.
	ErrJobNonePublished = errors.New("no job declares a url in run.toml — add one with `url = { port = \"PORT\" }`")

	// ErrDashboardJSON is returned when `wtm ui` is invoked with --output json.
	ErrDashboardJSON = errors.New("`wtm ui` has no --output json form — the dashboard cannot be driven by an agent; use `wtm list --output json`")
)

// ErrDaemonVersionMismatch is a run daemon built from another version of wtm
// holding the socket. It is the daemon that runs the jobs, so its behaviour is
// the one that applies, whatever the client's version fixed.
var ErrDaemonVersionMismatch = errors.New("run daemon version mismatch")

var (
	// ErrUpgradeFromSource is returned when wtm upgrade runs on a binary built
	// from source, where no published release corresponds to what is installed.
	ErrUpgradeFromSource = errors.New(UpgradeSourceHint)

	// ErrUpgradeNotWritable is returned when the running binary cannot be replaced
	// because its directory is not writable by the current user.
	ErrUpgradeNotWritable = errors.New("cannot write to the wtm binary — re-run with sudo")

	// ErrChecksumMismatch is returned when a downloaded release archive does not
	// match the SHA256 published in checksums.txt. Nothing is written.
	ErrChecksumMismatch = errors.New("downloaded archive failed checksum verification")

	// ErrReleaseAssetMissing is returned when the release carries no archive for
	// the running platform.
	ErrReleaseAssetMissing = errors.New("no release asset for this platform")

	// ErrReleaseNotFound is returned when the requested release tag does not exist.
	ErrReleaseNotFound = errors.New("release not found")
)
