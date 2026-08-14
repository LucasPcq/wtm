// Package domain defines shared types, constants, and errors for the wtm CLI.
package domain

const (
	// AppName is the canonical name of the CLI binary.
	AppName = "wtm"

	// Version is the current release version, overridden at build time via ldflags.
	Version = "dev"

	// ExitCodeOK indicates successful execution.
	ExitCodeOK = 0

	// ExitCodeError indicates a generic runtime error.
	ExitCodeError = 1

	// ExitCodeUsage indicates invalid usage or bad input.
	ExitCodeUsage = 2

	// Granular exit codes let LLM agents branch precisely on failure cause.
	ExitCodeWorktreeExists    = 10 // a worktree or its path already exists
	ExitCodeBranchNotFound    = 11 // the requested branch does not exist locally
	ExitCodeConfigNotFound    = 12 // the repo has no wtm config (run `wtm init`)
	ExitCodeServiceNotFound   = 14 // the referenced job is not declared in run.toml
	ExitCodeExtractConflict   = 15 // selected changes do not apply cleanly onto the target worktree
	ExitCodeRunNotInitialized = 16 // the run module is not initialized (run `wtm run init`)

	// StateDirName is the wtm state directory inside the git common dir
	// (i.e. <git-common-dir>/wtm/). Never committed — git ignores .git/.
	StateDirName = "wtm"

	// WorktreesSubdir is the subdirectory under the state dir that holds
	// per-worktree metadata: <state-dir>/worktrees/<encoded-branch>/.
	WorktreesSubdir = "worktrees"

	// ConfigFileName is the project-level config file name (inside <state-dir>/).
	ConfigFileName = "config.toml"

	// GlobalConfigDir is the subdirectory under ~/.config for wtm.
	GlobalConfigDir = "wtm"

	// GlobalConfigFile is the user-level config file name.
	GlobalConfigFile = "config.toml"

	// DefaultBasePath is the default directory for worktrees, relative to project root.
	// One level up so worktrees are created outside the main repo directory.
	DefaultBasePath = "../.trees"

	// DefaultBaseBranch is the default base branch for new worktrees.
	DefaultBaseBranch = "main"

	// DefaultEnvStrategy is the default .env provisioning strategy.
	DefaultEnvStrategy = EnvStrategyExample

	// EnvFileName is the canonical env value file — the copy target of any template.
	EnvFileName = ".env"

	// EnvLocalFileName is the machine-local override. Detected as a value file
	// (worktrees share a machine, so a local value is legitimately shareable) and
	// flagged local for informational display — never excluded from detection.
	EnvLocalFileName = ".env.local"

	// Committed-schema template suffixes recognized on a `.env` file. `.example`
	// is the dominant ecosystem convention; the rest are real but minority.
	EnvTemplateSuffixExample  = ".example"
	EnvTemplateSuffixDist     = ".dist"
	EnvTemplateSuffixSample   = ".sample"
	EnvTemplateSuffixTemplate = ".template"
	EnvTemplateSuffixTmpl     = ".tmpl"

	// `wtm env` value-source display labels — the single source a worktree's env
	// values come from, named by its strategy and shown in the drift report header.
	// `example` reads placeholders from the template only; `main` reads the main
	// worktree; `parent` reads the parent worktree only (main solely when the parent
	// has no local worktree at all).
	EnvSourceLabelTemplate = "template"
	EnvSourceLabelMain     = "main"
	EnvSourceLabelParent   = "parent worktree"
	// EnvSourceLabelNone is shown when the strategy's value source has no .env file at
	// all (and no fallback either) — nothing to sync from, so keys come from the
	// template as placeholders. Typical on a fresh project before any .env is created.
	EnvSourceLabelNone = "template (no .env to sync from)"

	// Tokens recognized by the .env content parser (internal/rules/env_parse.go).
	EnvCommentPrefix = "#"
	EnvExportPrefix  = "export "
	EnvAssign        = "="
	EnvQuoteDouble   = '"'
	EnvQuoteSingle   = '\''

	// DefaultShell is the default shell for integration.
	DefaultShell = ShellZsh

	// ShellInitCommand is the single source of truth for the shell-integration
	// eval line; both the multi-line hint and the init recap bullet compose it.
	ShellInitCommand = "eval \"$(wtm shell-init)\""

	// MsgShellInitHint tells the user how to set up shell integration.
	MsgShellInitHint = "Add this to your shell config:\n\n  " + ShellInitCommand

	// Lockfile names for package manager detection.
	LockfilePnpm = "pnpm-lock.yaml"
	LockfileNpm  = "package-lock.json"
	LockfileYarn = "yarn.lock"
	LockfileGo   = "go.mod"
	LockfilePip  = "requirements.txt"

	// Install commands per package manager.
	InstallCommandPnpm = "pnpm install"
	InstallCommandNpm  = "npm install"
	InstallCommandYarn = "yarn install"
	InstallCommandGo   = "go mod download"
	InstallCommandPip  = "pip install -r requirements.txt"

	// Workspace declaration files — their presence means one root install already
	// covers every package in the repo, so no per-package install is seeded.
	WorkspaceFilePnpm  = "pnpm-workspace.yaml"
	WorkspaceFileGo    = "go.work"
	WorkspaceFileTurbo = "turbo.json"
	WorkspaceFileNx    = "nx.json"
	WorkspaceFileLerna = "lerna.json"

	// PackageJSONFile is the npm manifest, used for script discovery and as the
	// marker that a globbed workspace pattern matched a real package.
	PackageJSONFile = "package.json"

	// MonorepoScanMaxDepth caps how deep sub-project discovery and globstar
	// workspace patterns descend below the project root (1 = direct children).
	MonorepoScanMaxDepth = 3

	// Directory names excluded from every project scan.
	ScanSkipNodeModules = "node_modules"
	ScanSkipTrees       = ".trees"
	ScanSkipGit         = ".git"
	ScanSkipVendor      = "vendor"
	ScanSkipDist        = "dist"

	// EnvGoFile is the environment variable used by the shell wrapper to pass the go-file path.
	EnvGoFile = "WTM_GO_FILE"

	// Flag names.
	FlagFrom       = "from"
	FlagFF         = "ff"
	FlagEnvFrom    = "env-from"
	FlagForce      = "force"
	FlagBase       = "base"
	FlagExclusive  = "exclusive"
	FlagParallel   = "parallel"
	FlagDetach     = "detach"
	FlagProfile    = "profile"
	FlagOutput     = "output"
	FlagYes        = "yes"
	FlagAll        = "all"
	FlagGlobal     = "global"
	FlagMerge      = "merge"
	FlagReplace    = "replace"
	FlagMine       = "mine"
	FlagReview     = "review"
	FlagCmd        = "cmd"
	FlagKind       = "kind"
	FlagStop       = "stop"
	FlagCwd        = "cwd"
	FlagJobs       = "jobs"
	FlagDefault    = "default"
	FlagTo         = "to"
	FlagKeep       = "keep"
	FlagFiles      = "files"
	FlagOnConflict = "on-conflict"

	// `wtm env` flags. FlagFrom (source override), FlagOnConflict (keep/overwrite),
	// FlagYes and FlagOutput are shared with other commands. FlagMode selects
	// add/refresh, FlagCheck is the read-only drift report, FlagPrune drops orphans.
	FlagMode  = "mode"
	FlagCheck = "check"
	FlagPrune = "prune"

	// FlagReparentChildren opts in (non-interactively) to reparenting the orphaned
	// children of a cleaned worktree onto its grandparent. In interactive mode the
	// command proposes this with a recap and an explicit confirmation instead.
	FlagReparentChildren = "reparent-children"

	// On-conflict modes for `extract`.
	OnConflictAbort   = "abort"
	OnConflictResolve = "resolve"

	// Status codes of the XY field of `git status --porcelain`.
	PorcelainUntracked = "??"
	PorcelainIgnored   = "!!"
	PorcelainRename    = "R"
	PorcelainCopy      = "C"
	PorcelainDeleted   = "D"

	// PorcelainFieldSep separates the records of `git status --porcelain -z`.
	PorcelainFieldSep = "\x00"

	// PorcelainPathOffset is where the path starts in a record: the two-character
	// XY status field plus its trailing space.
	PorcelainPathOffset = 3

	// init flags (non-interactive bootstrap).
	FlagIfNotExists    = "if-not-exists"
	FlagNonInteractive = "non-interactive"
	FlagShell          = "shell"
	FlagBasePath       = "base-path"
	FlagBaseBranch     = "base-branch"
	FlagEnvStrategy    = "env-strategy"
	FlagInstallCommand = "install-command"
	FlagCleanCommand   = "clean-command"

	// init skip flags — opt out of optional config sections (non-interactive).
	FlagSkipEnv   = "skip-env"
	FlagSkipHooks = "skip-hooks"
	FlagSkipClean = "skip-clean"

	// init wizard section gate choices — whether to configure or skip a section.
	WizardChoiceConfigure = "configure"
	WizardChoiceSkip      = "skip"

	// SkipMarkerComment is the leading comment written into a config.toml section
	// the user skipped during init. The config template emits it (followed by
	// section-specific guidance) so a skipped section stays valid but inert.
	SkipMarkerComment = "# Skipped during init."

	// FlagOnly re-runs init for specific sections only (re-init / re-detect).
	FlagOnly = "only"

	// Init section identifiers — used by `wtm init --only <section>`.
	SectionEnv       = "env"
	SectionHooks     = "hooks"
	SectionServices  = "services"
	SectionWorktrees = "worktrees"

	// sync flags — cascade rebase of worktrees onto their recorded parent.
	FlagDryRun       = "dry-run"
	FlagPush         = "push"
	FlagNoPush       = "no-push"
	FlagKeepConflict = "keep-conflict"

	// prune flags — batch removal of finished worktrees.
	FlagMerged  = "merged"
	FlagClosed  = "closed"
	FlagGone    = "gone"
	FlagNoFetch = "no-fetch"

	// FlagValidate makes `config show` validate the config instead of printing it.
	FlagValidate = "validate"

	// Prune candidate reasons — the category that made a worktree prunable,
	// emitted in the prune result (JSON + text recap). All are GitHub/remote
	// truth: a merged PR (--merged), a PR closed without merging (--closed), or a
	// deleted upstream branch (--gone).
	PruneReasonPRMerged = "pr_merged"
	PruneReasonPRClosed = "pr_closed"
	PruneReasonGone     = "gone"

	// Prune skip reasons — why a matching worktree was not removed. The current
	// worktree is not among them: prune removes it (like clean) and redirects the
	// shell to the base repo afterwards. Dirty/Unpushed/OpenPR mirror clean's
	// unsafe-to-remove checks: they skip unless --force is passed.
	PruneSkipBase     = "base_branch"
	PruneSkipMain     = "main_worktree"
	PruneSkipDirty    = "dirty"
	PruneSkipUnpushed = "unpushed"
	PruneSkipOpenPR   = "open_pr"

	// Script classification keywords for package.json → run.toml mapping.
	// A script is classified as a long-running service when its name matches
	// one of these keywords exactly, as a prefix ("<kw>:"), or as a suffix (":<kw>").
	ScriptKeyDev   = "dev"
	ScriptKeyStart = "start"
	ScriptKeyServe = "serve"
	ScriptKeyWatch = "watch"

	// FlagWithPRs includes GitHub PR info in non-interactive worktree listings.
	// PRs are fetched lazily (streamed) in interactive mode, but a pipe/JSON
	// consumer can't stream, so the fetch is opt-in and blocking there.
	FlagWithPRs = "with-prs"

	// GHPRFields is the JSON field set passed to `gh pr list/view --json`. It
	// holds exactly what wtm consumes: PR identity, head/base branches, url, and
	// the fork flag (isCrossRepository).
	GHPRFields = "number,title,author,headRefName,baseRefName,url,isCrossRepository,isDraft"

	// GHPRFieldsWithState is the field set for the all-states PR listing used by
	// `wtm tree --with-prs`, which must surface merged/closed PRs (clean
	// candidates) — so it includes the PR state.
	GHPRFieldsWithState = "number,headRefName,url,state"

	// PR states, normalised to lowercase. PRInfo.State always holds one of these,
	// and output routes rendering on them. Centralised so a typo can't silently
	// degrade merged/closed display.
	PRStateOpen   = "open"
	PRStateMerged = "merged"
	PRStateClosed = "closed"

	// Checkout wizard badge texts: a PR whose branch already has a local
	// worktree ("linked") or that comes from a fork ("fork") is disabled.
	BadgeTextLinked = "linked"
	BadgeTextFork   = "fork"

	// BadgeTextRemote tags a remote-tracking branch (origin/*) offered as a
	// worktree start-point or parent in a branch picker.
	BadgeTextRemote = "remote"

	// BadgeTextBase and BadgeTextOrigin prefix a worktree's commit badges to name
	// the referential the arrows count against: "base ↑N" = commits ahead of the
	// parent/base branch; "origin ↑a ↓b" = divergence from origin/<branch>. Same
	// glyphs, two referentials — the labels disambiguate them.
	BadgeTextBase   = "base"
	BadgeTextOrigin = "origin"

	// Divergence badge glyphs: a local branch ahead of / behind its origin
	// counterpart is labelled with these in a branch picker (e.g. "↑2 ↓5").
	BadgeGlyphAhead  = "↑"
	BadgeGlyphBehind = "↓"

	// Divergence state labels: the string form of a DivergenceState emitted in
	// JSON output (worktree origin divergence) so agents can branch on it.
	DivergenceLabelUpToDate = "up-to-date"
	DivergenceLabelBehind   = "behind"
	DivergenceLabelAhead    = "ahead"
	DivergenceLabelDiverged = "diverged"

	// Status pill glyphs: the leading symbol of a worktree row's right-aligned
	// dirty/clean status pill.
	BadgeGlyphDirty = "⚠"
	BadgeGlyphClean = "✓"

	// KeyRefresh is the picker key that re-fetches origin and recomputes the
	// branch divergence badges.
	KeyRefresh = "r"

	// RemoteBranchPrefix is the short-name prefix of origin remote-tracking refs
	// ("origin/feature"). Used to strip/build remote refs and to detect whether a
	// picked start-point is remote.
	RemoteBranchPrefix = "origin/"

	// LoadingBranchesText labels the spinner shown while a branch picker fetches
	// origin to refresh its divergence badges.
	LoadingBranchesText = "Fetching branches…"

	// LoadingWorktreesText labels the spinner shown while a worktree list fetches
	// origin to refresh its divergence badges.
	LoadingWorktreesText = "Fetching worktrees…"

	// SummaryConfigDefault is the env-step summary shown when no explicit env
	// strategy is chosen and the project config default applies.
	SummaryConfigDefault = "config default"

	// Output format values for FlagOutput.
	OutputText = "text"
	OutputJSON = "json"
	// OutputMermaid renders a Mermaid flowchart (wtm tree only) — a diagram that
	// can be pasted into a PR or Notion as a shareable discussion artifact.
	OutputMermaid = "mermaid"

	// RunFileName is the run config file name (inside <state-dir>/).
	RunFileName = "run.toml"

	// ExperimentalRunNotice is the single source of truth for the "run is
	// experimental" wording. Reused by the run-init output, the not-initialized
	// guard, and the mention printed at the end of `wtm init`, so the caveat
	// stays consistent everywhere the run module surfaces.
	ExperimentalRunNotice = "`wtm run` is experimental — the workflow is still stabilizing and commands may change."

	// MsgRunInitHint points users at the dedicated command that configures the
	// run module, printed at the end of `wtm init` (which no longer configures
	// services itself).
	MsgRunInitHint = "Run services per worktree ? Configure them with `wtm run init` (experimental)."

	// MsgRelocateHint points users at `wtm relocate` to adopt/align worktrees that
	// existed before wtm. Printed unconditionally at the end of `wtm init` — we do
	// not probe for pre-existing worktrees, the hint is cheap and always relevant.
	MsgRelocateHint = "Worktrees created before wtm ? Adopt and align them with `wtm relocate`."

	// Init recap (LUC-125): labels and copy for the framed end-of-init recap
	// (accent-bar box + pill title) that summarizes the written config and lists
	// the next steps. RecapWidth is the fixed render width shared with `relocate`.
	RecapWidth = 80

	InitRecapTitleGlobal  = "Global config ready"
	InitRecapTitleProject = "Project ready"
	InitRecapNextSteps    = "Next steps"

	InitRecapLabelShell       = "shell"
	InitRecapLabelBasePath    = "base_path"
	InitRecapLabelBaseBranch  = "base_branch"
	InitRecapLabelEnvStrategy = "env_strategy"
	InitRecapLabelOnCreate    = "on_create"
	InitRecapLabelOnClean     = "on_clean"

	InitRecapValueSkipped         = "skipped"
	InitRecapValueSkippedTemplate = "skipped (template)"
	InitRecapHookMoreFmt          = "%s  (+%d more)"

	// Hook phase titles: shown as a bold section header above the streamed hook
	// output, so create/clean read as distinct phases instead of loose lines.
	HooksTitleOnCreate = "Running on_create hooks"
	HooksTitleOnClean  = "Running on_clean hooks"

	// create result recap labels (aligned "label   value" rows).
	CreateRecapLabelFrom = "from"
	CreateRecapLabelEnv  = "env"
	CreateRecapLabelPath = "path"

	// GoCommandFmt builds the jump-in command shown by every worktree-creating
	// command (create, extract, checkout): `wtm go <branch>`.
	GoCommandFmt = "wtm go %s"

	// Next-step bullets printed in the recap. Each is a single line so the
	// "Next steps" block stays flat (no cascading indentation).
	InitNextStepShell    = "Add to your shell config:  " + ShellInitCommand
	InitNextStepCreate   = "wtm create <branch>  — create a worktree to get started"
	InitNextStepRelocate = "wtm relocate  — adopt & align pre-existing worktrees"
	InitNextStepRunInit  = "wtm run init  — (experimental) configure per-worktree services"

	// SchemasDirName is the directory (inside <state-dir>/ or under the global
	// config dir) where `wtm schema dump` writes the JSON Schema files
	// that editors reference via the TOML `#:schema` directive.
	SchemasDirName = "schemas"

	// Job action result statuses emitted by `run *` JSON output.
	JobActionStarted = "started"
	JobActionStopped = "stopped"
	JobActionDone    = "done"
	JobActionError   = "error"
	JobActionAdded   = "added"
	JobActionRemoved = "removed"
	JobActionUpdated = "updated"

	// MetaFileName is the metadata file created per worktree inside
	// <state-dir>/worktrees/<branch>/.
	MetaFileName = "meta.json"

	// Cobra group IDs — one per section of the root --help output.
	CmdGroupWorktrees = "worktrees"
	CmdGroupNavigate  = "navigate"
	CmdGroupStack     = "stack"
	CmdGroupJobs      = "jobs"
	CmdGroupGitHub    = "github"
	CmdGroupSetup     = "setup"

	// Cobra group titles — the section headers rendered in the root --help output,
	// registered alongside their IDs so a rename touches one place.
	CmdGroupWorktreesTitle = "Worktrees:"
	CmdGroupNavigateTitle  = "Navigate:"
	CmdGroupStackTitle     = "Stacked branches:"
	CmdGroupJobsTitle      = "Dev jobs (experimental):"
	CmdGroupGitHubTitle    = "GitHub:"
	CmdGroupSetupTitle     = "Setup:"

	// CLI command names — used in Use: declarations and exec.Command(bin, …) call sites.
	// Centralised here so a rename is a single-file change with no silent breakage.
	CmdRun      = "run"
	CmdInit     = "init"
	CmdGo       = "go"
	CmdCreate   = "create"
	CmdClean    = "clean"
	CmdList     = "list"
	CmdSwitch   = "switch"
	CmdUp       = "up"
	CmdDown     = "down"
	CmdStart    = "start"
	CmdStop     = "stop"
	CmdLogs     = "logs"
	CmdPs       = "ps"
	CmdCheckout = "checkout"
	CmdExport   = "export"
	CmdImport   = "import"
	CmdJob      = "job"
	CmdProfile  = "profile"
	CmdAdd      = "add"
	CmdRm       = "rm"
	CmdEdit     = "edit"
	CmdExtract  = "extract"
	CmdSync     = "sync"
	CmdRelocate = "relocate"
	CmdReparent = "reparent"
	CmdTree     = "tree"
	CmdPrune    = "prune"
	CmdEnv      = "env"

	// MinWizardListHeight is the minimum number of rows reserved for a wizard
	// step's scrollable list. Completed-step summaries are bounded so they never
	// shrink the list below this, keeping the breadcrumb (which names the worktree
	// being acted on) on screen even after many steps. See LUC-85.
	MinWizardListHeight = 3

	// DaemonSocketName is the Unix socket filename for the service daemon.
	DaemonSocketName = "wtm.sock"

	// DaemonIdleTimeoutSeconds is how long the daemon waits with no services before auto-exit.
	DaemonIdleTimeoutSeconds = 30

	// DaemonStartTimeoutSeconds is how long to wait for the daemon to start.
	DaemonStartTimeoutSeconds = 5

	// CtrlCByte is the ASCII code for Ctrl+C, used for PTY detach.
	CtrlCByte byte = 0x03

	// JobAlreadyRunningSuffix is the tail of the daemon error returned when a
	// job is started while already running. Callers match on it to treat a
	// repeat start (e.g. re-running `run up` while services are up) as a benign
	// no-op rather than a failure that aborts the profile.
	JobAlreadyRunningSuffix = "is already running"

	// SyncConfirmPrompt is the confirmation question shown before running a sync
	// cascade, formatted with the number of worktrees to rebase. Shared by the
	// interactive picker's confirmation step and the non-picker confirm prompt.
	SyncConfirmPrompt = "Rebase %d worktree(s) onto their parents?"

	// SyncKeepConflictWarning explains the consequence of keeping conflicting
	// rebases in progress; shown on the sync confirmation when --keep-conflict is
	// active.
	SyncKeepConflictWarning = "On conflict the rebase is left in progress in its worktree (not aborted). " +
		"Several worktrees may be left mid-rebase; resolve each with git rebase --continue."

	// SyncPushPrompt is the push confirmation question shown after a successful
	// cascade, formatted with the number of pushable branches.
	SyncPushPrompt = "Push %d rebased branch(es) to origin?"

	// SyncPushWarning clarifies what pushing does and that declining skips it:
	// No or Esc leaves the rebased branches local.
	SyncPushWarning = "Force-pushes the rebased branches with --force-with-lease. " +
		"No or Esc skips the push — branches stay local."

	// SyncPlanComputing is the loading message shown while the sync plan preview is
	// computed asynchronously on entering the confirmation step.
	SyncPlanComputing = "Computing sync plan…"

	// Source-reconciliation and env-fallback prompts shared by the create and
	// extract flows — used both by the in-wizard confirmation steps and the
	// standalone confirms on the non-interactive --from path. Format verbs:
	// %s source branch, %d commit counts.

	// SourceFastForwardPrompt offers to fast-forward a behind-only source branch
	// to origin before creating the worktree (source, behind).
	SourceFastForwardPrompt = "%s is %d commit(s) behind origin — fast-forward it before creating?"
	// SourceFastForwardDescription explains what the fast-forward does. Declining
	// keeps the source as-is rather than aborting.
	SourceFastForwardDescription = "Updates your local branch to origin so the new worktree starts up to date. " +
		"Skipped if its worktree has uncommitted changes."
	// SourceDivergedPrompt warns that a diverged source can't be fast-forwarded and
	// asks whether to create from it anyway (source, ahead, behind).
	SourceDivergedPrompt = "%s has diverged from origin (%d ahead, %d behind) — create the worktree from it anyway?"
	// SourceDivergedWarning explains the consequence of a diverged source.
	SourceDivergedWarning = "It can't be fast-forwarded. The worktree starts from your local branch, missing commits " +
		"that are on origin — you may have to rebase or resolve conflicts later."
	// SourceProceedStalePrompt asks whether to create from a stale local source
	// after a fast-forward failed (source, behind).
	SourceProceedStalePrompt = "Create the worktree from local %s anyway? (behind origin by %d)"
	// SourceProceedStaleWarning reports why the fast-forward failed (cause).
	SourceProceedStaleWarning = "Couldn't fast-forward: %v"

	// EnvParentFallbackPrompt warns, before creating, that the "parent" env
	// strategy will source .env from main because the source has no local worktree
	// (source).
	EnvParentFallbackPrompt = "%s has no local worktree — copy .env from the main worktree instead of the parent?"
	// EnvParentFallbackWarning explains why the fallback happens.
	EnvParentFallbackWarning = "The \"parent\" env strategy needs the source branch checked out to copy its .env; " +
		"without a worktree it comes from main."

	// PruneReparentPrompt is the confirmation shown when a prune leaves child
	// worktrees that can be reparented onto their grandparent (count). Hosted as a
	// step of the prune picker so declining goes back rather than aborting.
	PruneReparentPrompt = "Reparent %d child worktree(s) onto their grandparent?"
	// PruneReparentIntro precedes the list of children a prune would otherwise
	// orphan, shown in the reparent confirmation.
	PruneReparentIntro = "These children would otherwise be left orphaned:"

	// CleanReparentPrompt is the confirmation shown when cleaning a worktree that
	// has children which can be reparented onto their grandparent (count,
	// grandparent). Hosted as a step of the clean confirm wizard.
	CleanReparentPrompt = "Reparent %d child worktree(s) onto %s?"
	// CleanReparentIntro precedes the list of children a clean would otherwise
	// orphan, shown in the reparent confirmation.
	CleanReparentIntro = "These children would otherwise be left orphaned:"

	// CleanForceHintFmt is the refusal shown when a worktree is unsafe to remove
	// without --force (branch, reason).
	CleanForceHintFmt = "worktree %s %s; pass --force to remove it anyway"
	// CleanSudoConfirmFmt is the confirmation title for the privileged `sudo rm -rf`
	// removal fallback (worktree path).
	CleanSudoConfirmFmt = "Force-delete %s with `sudo rm -rf`? (you may be prompted for your password)"

	// WizardCancelLabel is the constant final option on every wizard recap step —
	// the single explicit cancellation point (alongside Esc on the first step).
	// Kept identical across commands so "No, cancel" always reads and sits the same.
	WizardCancelLabel = "No, cancel"
	// WizardCancelValue is the sentinel carried by the WizardCancelLabel row; the
	// command layer maps a chosen WizardCancelValue to ErrUserAborted.
	WizardCancelValue = "__wtm_wizard_cancel__"
	// WizardRecapTitle heads the final recap step, marking it visually as the
	// synthesis and action point.
	WizardRecapTitle = "Review & confirm"

	// MultiSelectHint is the shared footer hint for multi-select wizard steps, kept
	// identical across commands (sync, prune, extract) so the controls always read
	// the same.
	MultiSelectHint = "Space to toggle, a to select all, / to filter, enter to confirm, esc to cancel."

	// PinnedSuffixDefault, PinnedSuffixBase, and PinnedSuffixDetected label the
	// pinned first row of a branch picker, sharing one leading-space convention.
	PinnedSuffixDefault  = " (default)"
	PinnedSuffixBase     = " (base)"
	PinnedSuffixDetected = " (detected)"
)

// EnvTemplateSuffixes are the committed-schema template suffixes recognized on a
// `.env` file, in priority order — the first match is pinned when several templates
// exist for the same target (e.g. `.env.example` wins over `.env.dist`).
var EnvTemplateSuffixes = []string{
	EnvTemplateSuffixExample,
	EnvTemplateSuffixDist,
	EnvTemplateSuffixSample,
	EnvTemplateSuffixTemplate,
	EnvTemplateSuffixTmpl,
}
