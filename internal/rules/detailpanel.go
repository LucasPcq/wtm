package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type VitalChipsParams struct {
	Status       domain.WorktreeStatus
	LastCommitAt time.Time
	Now          time.Time
}

// VitalChips builds the vital strip. State comes first — it is the fastest
// read — and it is the only coloured chip. `created` never appears here: it
// belongs to LINKS.
func VitalChips(params VitalChipsParams) []domain.Chip {
	chips := []domain.Chip{stateChip(params.Status)}

	if params.Status.CommitsAhead > 0 {
		chips = append(chips, domain.Chip{
			Text: fmt.Sprintf(domain.ChipBaseFmt, params.Status.CommitsAhead),
			Kind: domain.ChipKindNeutral,
		})
	}
	if origin := originChipText(params.Status); origin != "" {
		chips = append(chips, domain.Chip{Text: origin, Kind: domain.ChipKindNeutral})
	}
	if age := RelativeAge(RelativeAgeParams{At: params.LastCommitAt, Now: params.Now}); age != "" {
		chips = append(chips, domain.Chip{
			Text: fmt.Sprintf(domain.ChipActiveFmt, age),
			Kind: domain.ChipKindNeutral,
		})
	}
	return chips
}

// stateChip: a paused rebase outranks "dirty". A mid-rebase branch is always
// dirty too, but the rebase is what is actionable.
func stateChip(status domain.WorktreeStatus) domain.Chip {
	if status.RebaseInProgress {
		return domain.Chip{Text: domain.ChipRebasing, State: true, Kind: domain.ChipKindRebasing}
	}
	if status.IsDirty {
		return domain.Chip{Text: domain.ChipDirty, State: true, Kind: domain.ChipKindDirty}
	}
	return domain.Chip{Text: domain.ChipClean, State: true, Kind: domain.ChipKindClean}
}

func originChipText(status domain.WorktreeStatus) string {
	switch status.OriginState {
	case domain.DivergenceUnknown:
		// No origin counterpart to compare against: nothing to show.
		return ""
	case domain.DivergenceUpToDate:
		// Same commit as origin is the non-event the strip must not recite.
		return ""
	case domain.DivergenceDiverged:
		return fmt.Sprintf(domain.ChipOriginDivergedFmt, status.OriginAhead, status.OriginBehind)
	case domain.DivergenceAhead:
		return fmt.Sprintf(domain.ChipOriginAheadFmt, status.OriginAhead)
	case domain.DivergenceBehind:
		return fmt.Sprintf(domain.ChipOriginBehindFmt, status.OriginBehind)
	default:
		return ""
	}
}

type DetailSectionsParams struct {
	Status domain.WorktreeStatus
	Detail domain.WorktreeDetail
	PR     *domain.PRInfo
	// PRUnavailable names why PR data could not be read (e.g. the GitHub CLI is
	// missing or not authenticated), empty when it is fine. It is a plain reason
	// string so the renderer stays dumb: rules/ still decides whether REVIEW
	// shows, the caller only supplies why the read failed.
	PRUnavailable string
	Parent        string
	// RunConfig is what the project declares it can run, and Jobs what the daemon
	// currently holds. Together they are the RUN section: a declared job that is
	// not in Jobs is down, which is an answer and not an absence.
	RunConfig domain.RunConfig
	Jobs      []domain.JobInfo
	// Addresses is where each declared job answers in this worktree, keyed by
	// job name. It follows the poll like Jobs does, not the lazily loaded
	// Detail: an address is a property of the worktree's offset, and the two
	// must not be able to disagree.
	Addresses map[string]domain.JobAddress
	// Expanded keys the runners whose children are unfolded; see
	// runSectionParams.Expanded.
	Expanded map[string]bool
	// AddressNote is what has to be said about those addresses, empty when the
	// worktree's .env answers on the names its jobs publish.
	AddressNote string
	// DetailLoaded is false on the very first render for a branch, before its
	// WorktreeDetail has ever landed (§8 state 3). CHANGES and ACTIVITY — the
	// two sections that depend on Detail — render a single "loading…"
	// placeholder line instead of vanishing, so LINKS does not jump once the
	// real data arrives. CHANGES is only expected when WorktreeStatus already
	// says the tree is dirty; ACTIVITY is expected unconditionally, since a
	// branch with no commit history at all is not a case this panel needs to
	// special-case.
	DetailLoaded bool
	Height       int
	// Now anchors every relative age this call renders (ACTIVITY's "… ago"), so
	// the whole panel is reproducible from its inputs instead of reading the
	// wall clock mid-build.
	Now time.Time
}

// DetailSections emits the sections in a fixed order, and only the ones that
// have something to say: a section's position varies between worktrees, its
// rank never does.
//
// CHANGES and ACTIVITY each get a fixed row cap (listBudgets) rather than a
// share of whatever height happens to be left over: leftover height simply
// stays empty. FitSections is what reacts to a real height shortage, by
// dropping whole sections — never by stretching or shrinking a list's cap.
func DetailSections(params DetailSectionsParams) []domain.DetailSection {
	var review *domain.DetailSection
	if params.PR != nil || params.PRUnavailable != "" {
		section := reviewSection(reviewSectionParams{PR: params.PR, Unavailable: params.PRUnavailable})
		review = &section
	}
	links := linksSection(params)

	changesErr := familyFailure(params.Detail.Failures, domain.DetailFamilyChanges)
	commitsErr := familyFailure(params.Detail.Failures, domain.DetailFamilyCommits)
	diffErr := familyFailure(params.Detail.Failures, domain.DetailFamilyBranchDiff)
	wantChanges := changedCount(params.Detail.Changes) > 0 || changesErr != nil
	wantActivity := len(params.Detail.Commits) > 0 || commitsErr != nil
	if !params.DetailLoaded {
		wantChanges, wantActivity = params.Status.IsDirty, true
	}

	changesBudget, activityBudget := listBudgets(wantChanges, wantActivity)

	run := runSectionParams{
		Jobs:        params.RunConfig.Jobs,
		Infos:       params.Jobs,
		Expanded:    params.Expanded,
		Addresses:   params.Addresses,
		AddressNote: params.AddressNote,
		WorkDir:     params.Status.Path,
		Budget:      domain.DashboardDetailJobs,
		Now:         params.Now,
	}

	sections := make([]domain.DetailSection, 0, 5)
	if len(params.RunConfig.Jobs) > 0 {
		sections = append(sections, runSection(run))
	}
	if review != nil {
		sections = append(sections, *review)
	}
	if wantChanges {
		sections = append(sections, changesSection(changesSectionParams{
			Changes: params.Detail.Changes, Budget: changesBudget, Failure: changesErr, Loaded: params.DetailLoaded,
		}))
	}
	if wantActivity {
		sections = append(sections, activitySection(activitySectionParams{
			Commits: params.Detail.Commits, Budget: activityBudget, Now: params.Now, Failure: commitsErr, Loaded: params.DetailLoaded,
			Diff: params.Detail.BranchDiff, DiffFailure: diffErr,
		}))
	}
	sections = append(sections, links)
	if len(params.RunConfig.Jobs) == 0 {
		return sections
	}
	return growRunSection(growRunParams{Sections: sections, Run: run, Height: params.Height})
}

type growRunParams struct {
	Sections []domain.DetailSection
	Run      runSectionParams
	Height   int
}

// growRunSection spends the panel's leftover rows on RUN. The other lists keep
// a fixed cap because what they could show is unbounded — a worktree has any
// number of commits and changed files — but the jobs are a closed list the
// panel exists to show, and folding one away under twenty blank rows hides the
// address the reader came for. It only ever grows: a panel with no room left
// keeps the section it already had.
func growRunSection(params growRunParams) []domain.DetailSection {
	slack := params.Height - sectionsHeight(params.Sections)
	if slack <= 0 {
		return params.Sections
	}

	grown := params.Run
	grown.Budget = min(params.Run.Budget+slack, len(params.Run.Jobs))
	if grown.Budget <= params.Run.Budget {
		return params.Sections
	}

	for index, section := range params.Sections {
		if section.Key == domain.DetailSectionRun {
			params.Sections[index] = runSection(grown)
			break
		}
	}
	return params.Sections
}

// familyFailure reads a family's failure out of Failures without ever
// promoting a nil error into a "failed" reading: a family absent from the map,
// or present with a nil error, was read successfully.
func familyFailure(failures map[domain.DetailFamily]error, family domain.DetailFamily) error {
	if err, failed := failures[family]; failed && err != nil {
		return err
	}
	return nil
}

// failureLine is the shared §8 state 4 form: every family that failed to read
// says why, in the same words, instead of that family's section going quietly
// empty.
func failureLine(err error) string {
	return fmt.Sprintf(domain.DashboardUnavailableFmt, err)
}

func changedCount(changes domain.WorkingChanges) int {
	return changes.Modified + changes.Untracked + changes.Staged
}

// listBudgets gives each shown list a fixed maximum row count —
// DashboardDetailChanges for CHANGES, DashboardDetailCommits for ACTIVITY —
// instead of splitting whatever height happens to be left over between them.
// The previous rule handed the leftover to CHANGES on a dirty worktree and to
// ACTIVITY on a clean one, which read as randomness rather than a rule: five
// commits on a clean worktree, two plus "… 3 more" the moment it turns dirty,
// with nothing on screen explaining why. A fixed cap is predictable even when
// its reasoning is invisible; leftover height simply stays empty. A real
// height shortage is FitSections' job, by dropping a whole section, never by
// shrinking a list's cap.
func listBudgets(wantChanges, wantActivity bool) (changesBudget, activityBudget int) {
	if wantChanges {
		changesBudget = domain.DashboardDetailChanges
	}
	if wantActivity {
		activityBudget = domain.DashboardDetailCommits
	}
	return changesBudget, activityBudget
}

// splitBudget divides a list of `total` items into what's shown and what's
// folded into a "… N more" line, reserving one row for that line once the
// budget is exceeded.
func splitBudget(total, budget int) (shown, more int) {
	if budget <= 0 {
		return 0, total
	}
	if total <= budget {
		return total, 0
	}
	return budget - 1, total - (budget - 1)
}

type reviewSectionParams struct {
	PR          *domain.PRInfo
	Unavailable string
}

// reviewSection reads either a real PR or a reason PR data could not be read
// — never both, and the caller only ever supplies one (DetailSections only
// builds this section when at least one is set). A failed read renders the
// same "unavailable — reason" form as the LINKS "Env" field (§8 state 4): an
// absence caused by a broken tool must never look like an absence caused by
// there being nothing.
func reviewSection(params reviewSectionParams) domain.DetailSection {
	if params.PR == nil {
		line := fmt.Sprintf(domain.DashboardUnavailableFmt, params.Unavailable)
		return domain.DetailSection{Key: domain.DetailSectionReview, Title: domain.DetailSectionReview, Lines: []string{line}}
	}

	lines := []string{domain.DetailListIndent + fmt.Sprintf(domain.DetailReviewHeaderFmt, params.PR.Number, params.PR.Title, params.PR.State)}
	if second := reviewChecksLine(params.PR.Checks, params.PR.ReviewDecision); second != "" {
		lines = append(lines, domain.DetailListIndent+second)
	}
	return domain.DetailSection{Key: domain.DetailSectionReview, Title: domain.DetailSectionReview, Lines: lines}
}

// reviewChecksLine renders the REVIEW section's second line: a checks
// fragment (omitted when no check ever ran) and a review-decision fragment
// (omitted when nothing has been decided yet), joined only when both apply.
func reviewChecksLine(checks domain.PRChecks, decision string) string {
	var fragments []string
	if checks.Passed+checks.Failed+checks.Pending > 0 {
		fragments = append(fragments, checksFragment(checks))
	}
	if label := reviewDecisionLabel(decision); label != "" {
		fragments = append(fragments, fmt.Sprintf(domain.DetailReviewDecisionFmt, label))
	}
	return strings.Join(fragments, domain.DashboardMetaSeparator)
}

func checksFragment(checks domain.PRChecks) string {
	fragment := fmt.Sprintf(domain.DetailChecksFmt, checks.Passed, checks.Failed)
	if checks.Pending > 0 {
		fragment += fmt.Sprintf(domain.DetailChecksPendingFmt, checks.Pending)
	}
	return fragment
}

func reviewDecisionLabel(decision string) string {
	switch decision {
	case domain.GHReviewDecisionApproved:
		return domain.DetailReviewDecisionApproved
	case domain.GHReviewDecisionChangesRequested:
		return domain.DetailReviewDecisionChangesRequested
	case domain.GHReviewDecisionReviewRequired:
		return domain.DetailReviewDecisionReviewRequired
	default:
		return ""
	}
}

type changesSectionParams struct {
	Changes domain.WorkingChanges
	Budget  int
	// Failure, when set, replaces the file list with why it could not be read.
	Failure error
	// Loaded is false on the very first render (§8 state 3): the section
	// renders a single loading placeholder instead of a guess at its content.
	Loaded bool
}

func changesSection(params changesSectionParams) domain.DetailSection {
	section := domain.DetailSection{Key: domain.DetailSectionChanges, Title: domain.DetailSectionChanges}
	switch {
	case !params.Loaded:
		section.Lines = []string{domain.DashboardLoadingField}
		return section
	case params.Failure != nil:
		section.Lines = []string{failureLine(params.Failure)}
		return section
	}

	var lines []string
	shown, more := splitBudget(len(params.Changes.Files), params.Budget)
	for _, file := range params.Changes.Files[:shown] {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailFileFmt, fileGlyph(file), file.Path))
	}
	if more > 0 {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailMoreFmt, more))
	}
	section.TitleRight = changesSummary(params.Changes)
	section.Lines = lines
	return section
}

func changesSummary(changes domain.WorkingChanges) string {
	var parts []string
	if changes.Modified > 0 {
		parts = append(parts, fmt.Sprintf(domain.ChangesModifiedFmt, changes.Modified))
	}
	if changes.Untracked > 0 {
		parts = append(parts, fmt.Sprintf(domain.ChangesUntrackedFmt, changes.Untracked))
	}
	if changes.Staged > 0 {
		parts = append(parts, fmt.Sprintf(domain.ChangesStagedFmt, changes.Staged))
	}
	summary := strings.Join(parts, domain.DashboardMetaSeparator)

	diff := diffStatText(domain.DiffStat{Insertions: changes.Insertions, Deletions: changes.Deletions})
	if diff == "" {
		return summary
	}
	if summary == "" {
		return diff
	}
	return summary + domain.DetailListIndent + diff
}

// diffStatText renders a diff's volume ("+214 −38"), or "" when there is
// nothing to report — never a fabricated "+0 −0" for an unread diff.
func diffStatText(stat domain.DiffStat) string {
	if stat.Insertions == 0 && stat.Deletions == 0 {
		return ""
	}
	return fmt.Sprintf(domain.ChangesDiffStatFmt, stat.Insertions, stat.Deletions)
}

// fileGlyph reads the porcelain XY code as-is: the worktree column (Y) wins
// when set, since it's what you'd act on next; the index column (X) is the
// fallback for a purely staged change.
func fileGlyph(file domain.PorcelainEntry) string {
	if file.Status == domain.PorcelainUntracked {
		return domain.DetailUntrackedGlyph
	}
	if len(file.Status) < 2 {
		return domain.DetailUntrackedGlyph
	}
	if rune(file.Status[1]) != domain.PorcelainUnmodified {
		return string(file.Status[1])
	}
	return string(file.Status[0])
}

type activitySectionParams struct {
	Commits []domain.CommitSummary
	Budget  int
	Now     time.Time
	// Failure, when set, replaces the commit list with why it could not be read.
	Failure error
	// Loaded is false on the very first render (§8 state 3): the section
	// renders a single loading placeholder instead of a guess at its content.
	Loaded bool
	// Diff is the branch's committed volume against its base, rendered on the
	// title row — ACTIVITY's counterpart to CHANGES' uncommitted volume.
	Diff domain.DiffStat
	// DiffFailure, when set, replaces Diff on the title row with why it could
	// not be read: a failed read never fabricates a "+0 −0".
	DiffFailure error
}

func activitySection(params activitySectionParams) domain.DetailSection {
	section := domain.DetailSection{Key: domain.DetailSectionActivity, Title: domain.DetailSectionActivity}
	switch {
	case !params.Loaded:
		section.Lines = []string{domain.DashboardLoadingField}
		return section
	case params.Failure != nil:
		section.Lines = []string{failureLine(params.Failure)}
		return section
	}

	shown, more := splitBudget(len(params.Commits), params.Budget)
	lines := make([]string, 0, shown+1)
	for _, commit := range params.Commits[:shown] {
		lines = append(lines, domain.DetailListIndent+commitLine(commit, params.Now))
	}
	if more > 0 {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailMoreFmt, more))
	}
	section.TitleRight = activityDiffText(params.Diff, params.DiffFailure)
	section.Lines = lines
	return section
}

func activityDiffText(stat domain.DiffStat, err error) string {
	if err != nil {
		return failureLine(err)
	}
	if stat.FilesChanged == 0 {
		return diffStatText(stat)
	}
	files := fmt.Sprintf(domain.ActivityFilesChangedFmt, stat.FilesChanged)
	diff := diffStatText(stat)
	if diff == "" {
		return files
	}
	return files + domain.DetailListIndent + diff
}

// commitLine reuses commit.SHA as-is: infra.RecentCommits already asks git for
// its abbreviated `%h`, so re-truncating it here would only risk cutting an
// abbreviation git had to grow to stay unambiguous.
func commitLine(commit domain.CommitSummary, now time.Time) string {
	line := fmt.Sprintf(domain.DetailCommitFmt, commit.SHA, commit.Subject)
	if meta := commitMeta(commit, now); meta != "" {
		line += domain.DetailListIndent + meta
	}
	return line
}

// commitMeta never formats a zero CommitSummary.At as a date: RelativeAge
// already returns "" for it, meaning unknown, never the epoch.
func commitMeta(commit domain.CommitSummary, now time.Time) string {
	var parts []string
	if commit.Author != "" {
		parts = append(parts, commit.Author)
	}
	if age := RelativeAge(RelativeAgeParams{At: commit.At, Now: now}); age != "" {
		parts = append(parts, age)
	}
	return strings.Join(parts, domain.DashboardMetaSeparator)
}

func linksSection(params DetailSectionsParams) domain.DetailSection {
	var lines []string

	if params.Parent != "" {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelParent, params.Parent))
	}
	if len(params.Detail.Children) > 0 {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelChildren,
			strings.Join(params.Detail.Children, domain.DetailListSep)))
	}
	if age := RelativeAge(RelativeAgeParams{At: params.Status.CreatedAt, Now: params.Now}); age != "" {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelCreated, age))
	}
	if env := envLine(params.Detail); env != "" {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelEnv, env))
	}
	lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelPath, params.Status.Path))

	return domain.DetailSection{Key: domain.DetailSectionLinks, Title: domain.DetailSectionLinks, Lines: lines}
}

// envLine distinguishes three situations the LINKS "Env" field can be in: a
// family that failed to read says why; a legitimate absence (no env files
// declared) says so without an alert glyph; a configured family with no drift
// has nothing to say and is omitted, like an up-to-date origin.
func envLine(detail domain.WorktreeDetail) string {
	if err := familyFailure(detail.Failures, domain.DetailFamilyEnv); err != nil {
		return failureLine(err)
	}
	if !detail.EnvDrift.Configured {
		return domain.DashboardNotConfigured
	}
	return envDriftSummary(detail.EnvDrift)
}

func envDriftSummary(drift domain.EnvDriftSummary) string {
	var parts []string
	if drift.Missing > 0 {
		parts = append(parts, fmt.Sprintf(domain.EnvMissingFmt, drift.Missing))
	}
	if drift.Conflicting > 0 {
		parts = append(parts, fmt.Sprintf(domain.EnvConflictingFmt, drift.Conflicting))
	}
	if drift.Orphan > 0 {
		parts = append(parts, fmt.Sprintf(domain.EnvOrphanFmt, drift.Orphan))
	}
	return strings.Join(parts, domain.DashboardMetaSeparator)
}

type FitSectionsParams struct {
	Sections []domain.DetailSection
	Height   int
}

// FitSections drops sections from the end of DetailSectionDropOrder while the
// stack exceeds the height. The vital strip and the blockers line never come
// through here: they don't fall.
func FitSections(params FitSectionsParams) []domain.DetailSection {
	kept := params.Sections
	for sectionsHeight(kept) > params.Height && len(kept) > 0 {
		kept = dropLowestPriority(kept)
	}
	return kept
}

// sectionsHeight counts, per section, its title, the blank line under it and
// its body lines.
func sectionsHeight(sections []domain.DetailSection) int {
	total := 0
	for _, section := range sections {
		total += domain.DetailSectionChrome + len(section.Lines) + len(section.Rows)
	}
	return total
}

func dropLowestPriority(sections []domain.DetailSection) []domain.DetailSection {
	for i := len(domain.DetailSectionDropOrder) - 1; i >= 0; i-- {
		key := domain.DetailSectionDropOrder[i]
		for index, section := range sections {
			if section.Key == key {
				return append(sections[:index:index], sections[index+1:]...)
			}
		}
	}
	return nil
}

// jobsByBudgetPriority puts the jobs that are up first when the section cannot
// show them all. A running job carries an address and an uptime; one that is
// merely stopped carries neither, so folding it away costs the reader nothing —
// while folding a running one buried the very URL the panel is read for.
// Declared order is kept inside each group, and untouched when everything fits.
type jobsByBudgetPriorityParams struct {
	Jobs    []VisibleJob
	Folding bool
}

func jobsByBudgetPriority(params jobsByBudgetPriorityParams) []VisibleJob {
	if !params.Folding {
		return params.Jobs
	}

	ordered := make([]VisibleJob, 0, len(params.Jobs))
	for _, job := range params.Jobs {
		if job.State == JobStateUp {
			ordered = append(ordered, job)
		}
	}
	for _, job := range params.Jobs {
		if job.State != JobStateUp {
			ordered = append(ordered, job)
		}
	}
	return ordered
}

type runSectionParams struct {
	Jobs  []domain.JobConfig
	Infos []domain.JobInfo
	// Logged names the jobs that left output in this worktree's log directory.
	// It is the trace half of the visibility rule: without it a job the daemon
	// has dropped — every task that ever finished — would vanish along with the
	// only way to read what it wrote.
	// Expanded keys the runners whose children are unfolded. Folded is the
	// default: a runner holding six apps is one line until asked, which is what
	// keeps the section readable on a project that nests them.
	Expanded    map[string]bool
	Addresses   map[string]domain.JobAddress
	AddressNote string
	WorkDir     string
	Budget      int
	Now         time.Time
}

// runSection is one row per job that lives or left a trace in this worktree,
// never one per declaration: a project declaring fifteen jobs drew twelve rows
// reading "down", four of them tasks that never run at all, and the three that
// were actually up had to be found among them. What the project can run beyond
// that is a catalogue, closed under a "+ N declared" line rather than laid
// flat here. The count heads the section, so what it is worth is legible
// before its body is.
func runSection(params runSectionParams) domain.DetailSection {
	indexed := IndexedJobsByName(params.Infos, params.WorkDir)
	// A job a running runner holds has no row: its address is on the runner's,
	// and six apps listed under the one process that started them is the noise
	// this section exists to spare the reader.
	declared := withoutHeldJobs(params.Jobs, upOnly(indexed))
	// No Traces: this section is about the present. What ran here days ago is
	// the logs view's subject, and reading it here turned the panel into the
	// worktree's archaeology — fifteen rows of which two were about today.
	visible, hidden := VisibleJobs(VisibleJobsParams{Jobs: declared, Up: indexed})

	shown, folded := splitBudget(len(visible), params.Budget)
	ordered := jobsByBudgetPriority(jobsByBudgetPriorityParams{Jobs: visible, Folding: folded > 0})

	up := CountUp(visible)
	rows := make([]domain.DetailRow, 0, shown)
	for index, job := range ordered {
		if index >= shown {
			continue
		}
		address := params.Addresses[job.Job.Name]
		rows = append(rows, jobRow(jobRowParams{
			Visible:  job,
			Address:  address,
			Expanded: params.Expanded[job.Job.Name],
			Now:      params.Now,
		}))
		if params.Expanded[job.Job.Name] {
			rows = append(rows, heldRows(job.Job.Name, address)...)
		}
	}
	if folded > 0 {
		rows = append(rows, domain.DetailRow{Cells: []domain.DetailCell{{
			Kind: domain.DetailCellNote, Text: fmt.Sprintf(domain.DetailMoreFmt, folded),
		}}})
	}
	// The catalogue line closes the section rather than opening it: it is the
	// least urgent thing here, and it must never push a running job down a row.
	if hidden > 0 {
		rows = append(rows, domain.DetailRow{Cells: []domain.DetailCell{{
			Kind: domain.DetailCellNote, Text: fmt.Sprintf(domain.DetailDeclaredMoreFmt, hidden),
		}}})
	}
	// Under the rows rather than beside one: it is the worktree that is
	// unsettled, and repeating it on every address would drown the addresses.
	if params.AddressNote != "" {
		rows = append(rows,
			domain.DetailRow{Cells: []domain.DetailCell{{Kind: domain.DetailCellGap}}},
			domain.DetailRow{Cells: []domain.DetailCell{{Kind: domain.DetailCellWarn, Text: params.AddressNote}}},
		)
	}

	return domain.DetailSection{
		Key:        domain.DetailSectionRun,
		Title:      domain.DetailSectionRun,
		TitleRight: runCount(up),
		Rows:       rows,
	}
}

// withoutHeldJobs drops the published jobs a running runner already answers
// for. Nothing is hidden while nothing holds it: a child whose runner is down
// keeps its row, which is where its address is written.
func withoutHeldJobs(jobs []domain.JobConfig, up map[string]domain.JobInfo) []domain.JobConfig {
	running := make(map[string]bool, len(up))
	for name := range up {
		running[name] = true
	}
	held := HeldBy(domain.RunConfig{Jobs: jobs}, running)
	if len(held) == 0 {
		return jobs
	}

	kept := make([]domain.JobConfig, 0, len(jobs))
	for _, job := range jobs {
		if !held[job.Name] {
			kept = append(kept, job)
		}
	}
	return kept
}

type jobRowParams struct {
	Visible VisibleJob
	Address domain.JobAddress
	// Expanded says the runner's children are showing under this row, which is
	// only about which way its fold marker points.
	Expanded bool
	Now      time.Time
}

// heldRows are the addresses a runner answers for, one row each, hanging off
// its own. Each carries its url, so each is a row a reader can click — which
// the single joined-and-truncated cell they used to share never was.
func heldRows(runner string, address domain.JobAddress) []domain.DetailRow {
	rows := make([]domain.DetailRow, 0, len(address.Held))
	for _, entry := range address.Held {
		rows = append(rows, domain.DetailRow{
			Key:   HeldRowKey(runner, entry.Job),
			Depth: 1,
			URL:   entry.URL,
			Cells: []domain.DetailCell{
				{Kind: domain.DetailCellName, Text: entry.Job},
				{Kind: domain.DetailCellAddress, Text: entry.URL},
			},
		})
	}
	return rows
}

// HeldRowKey names a child's row under the runner holding it. Qualified by the
// runner because the same app can be held by two of them, and a bare name would
// give one row two meanings.
func HeldRowKey(runner, child string) string { return runner + "/" + child }

// jobRow is the one place a job's row is shaped, shared by the detail panel and
// the run board. A job that is not up says nothing beyond its glyph and its
// name: its address is where it would answer, not where it does, and an uptime
// on it would date a run that is over.
func jobRow(params jobRowParams) domain.DetailRow {
	job, state := params.Visible.Job, params.Visible.State
	cells := []domain.DetailCell{
		{Kind: domain.DetailCellGlyph, Text: JobStateGlyph(state)},
		{Kind: domain.DetailCellName, Text: job.Name},
	}
	if state != JobStateUp {
		return domain.DetailRow{Key: job.Name, Cells: cells}
	}

	if address := JobAddressText(params.Address); address != "" {
		cells = append(cells, domain.DetailCell{Kind: domain.DetailCellAddress, Text: address})
	}
	// A runner with children folds; anything else has nothing to open.
	if len(params.Address.Held) > 0 {
		return domain.DetailRow{
			Key: job.Name, Cells: append(cells, foldCell(params.Expanded)),
			Up: true, URL: params.Address.URL, Fold: true, Folded: !params.Expanded,
		}
	}
	if uptime := VisibleJobUptime(params.Visible, params.Now); uptime != "" {
		cells = append(cells, domain.DetailCell{Kind: domain.DetailCellMeta, Text: uptime})
	}
	return domain.DetailRow{Key: job.Name, Cells: cells, Up: true, URL: params.Address.URL}
}

func foldCell(expanded bool) domain.DetailCell {
	glyph := domain.DetailHeldShutGlyph
	if expanded {
		glyph = domain.DetailHeldOpenGlyph
	}
	return domain.DetailCell{Kind: domain.DetailCellFold, Text: glyph}
}

// upOnly narrows the daemon's index for this worktree to what is actually up.
// withoutHeldJobs asks about runners that are holding their children right now,
// which a stopped one is not doing.
func upOnly(indexed map[string]domain.JobInfo) map[string]domain.JobInfo {
	up := make(map[string]domain.JobInfo, len(indexed))
	for name, info := range indexed {
		if IsJobUp(info.Status) {
			up[name] = info
		}
	}
	return up
}

// JobAddressText is the url when the job publishes one, its ports otherwise —
// never both: a url already carries the port, and printing the two is the same
// fact twice, which is what made the section read as columns of noise. Shared
// with the run view, so a job reads the same in every surface.
//
// A runner publishing nothing of its own has no address of its own either. The
// names it holds used to be joined onto its line, which was wrong everywhere it
// was read: six urls behind a "," ran past the panel and were cut, taking four
// of them with it, on a cell that could not even be clicked. They are rows of
// their own now (HeldSummaryText and the runner's children), and one line each
// where nothing folds (HeldAddressLines).
func JobAddressText(address domain.JobAddress) string {
	if address.URL != "" {
		return address.URL
	}
	// Before the ports, not after: a runner is given its children's ports, so the
	// numbers on it are theirs and reading them back would name the runner after
	// jobs it is not.
	if len(address.Held) > 0 {
		return HeldSummaryText(len(address.Held))
	}
	names := make([]string, 0, len(address.Ports))
	for _, port := range address.Ports {
		names = append(names, fmt.Sprintf(domain.DetailJobPortFmt, port))
	}
	return strings.Join(names, domain.DetailListSep)
}

// HeldSummaryText stands for the addresses a runner answers for on the one line
// it gets. It counts rather than lists: which of six urls to show first is a
// choice run.toml does not express, so the count is the only honest summary.
func HeldSummaryText(held int) string {
	if held == 0 {
		return ""
	}
	return fmt.Sprintf(domain.DetailHeldCountFmt, held)
}

func runCount(up int) string {
	if up == 0 {
		return domain.DetailRunNothing
	}
	return fmt.Sprintf(domain.DetailRunUpCountFmt, up)
}
