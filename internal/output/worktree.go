// Package output formats and prints results. It contains zero decision logic.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatWorktreeListParams holds inputs for rendering the worktree list.
type FormatWorktreeListParams struct {
	Statuses     []domain.WorktreeStatus
	ActiveBranch string
	PRInfos      []domain.PRInfo
	Services     []domain.JobInfo
}

// FormatWorktreeList renders a list of worktree statuses as an aligned table string.
func FormatWorktreeList(params FormatWorktreeListParams) string {
	if len(params.Statuses) == 0 {
		return "No worktrees found."
	}

	rows := buildRows(params.Statuses, params.ActiveBranch, params.PRInfos, params.Services)
	widths := columnWidths(rows)

	var builder strings.Builder
	for _, row := range rows {
		line := formatRow(row, widths)
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return builder.String()
}

type row struct {
	branch   string
	tag      string
	pr       string
	services string
	ahead    string
	origin   string
	status   string
}

func buildRows(statuses []domain.WorktreeStatus, activeBranch string, prs []domain.PRInfo, svcs []domain.JobInfo) []row {
	rows := make([]row, 0, len(statuses))
	for _, s := range statuses {
		r := row{
			branch:   styles.Bold.Render(s.Branch),
			tag:      formatTag(s.IsParent, s.Branch == activeBranch),
			pr:       formatPRTag(s.Branch, prs),
			services: formatServicesTag(s.Path, svcs),
			ahead:    formatAhead(s.CommitsAhead),
			origin:   formatOrigin(s),
			status:   formatWorktreeState(s),
		}
		rows = append(rows, r)
	}
	return rows
}

func formatPRTag(branch string, prs []domain.PRInfo) string {
	for _, pr := range prs {
		if pr.Branch == branch {
			return styles.Success.Render(fmt.Sprintf("PR #%d", pr.Number))
		}
	}
	return ""
}

func formatServicesTag(worktreePath string, svcs []domain.JobInfo) string {
	for _, svc := range svcs {
		if svc.WorkDir == worktreePath && rules.IsJobUp(svc.Status) {
			return styles.Success.Render("services")
		}
	}
	return ""
}

func formatTag(isParent bool, isActive bool) string {
	tags := ""
	if isParent {
		tags = styles.Muted.Render("(parent)")
	}
	if isActive {
		active := styles.Success.Render(domain.WorktreeActiveTag)
		if tags != "" {
			tags += "  " + active
		} else {
			tags = active
		}
	}
	return tags
}

// formatWorktreeState renders the status column. A paused rebase takes
// precedence over the generic dirty flag (a mid-rebase tree is always dirty, but
// "rebasing" is the actionable signal).
func formatWorktreeState(s domain.WorktreeStatus) string {
	if s.RebaseInProgress {
		return styles.Warning.Render("⚠ rebasing")
	}
	if s.IsDirty {
		return styles.Warning.Render("⚠ dirty")
	}
	return styles.Success.Render("✓ clean")
}

// formatAhead renders the commits-ahead-of-base column, labelled "base ↑N" so it
// is not confused with the origin-divergence column. Zero renders empty.
func formatAhead(count int) string {
	if count == 0 {
		return ""
	}
	return styles.Muted.Render(fmt.Sprintf("%s %s%d", domain.BadgeTextBase, domain.BadgeGlyphAhead, count))
}

// formatOrigin renders the origin-divergence column, labelled "origin ↑a ↓b" and
// colored by state. Up-to-date and unknown (no origin counterpart) render empty.
func formatOrigin(s domain.WorktreeStatus) string {
	switch s.OriginState {
	case domain.DivergenceBehind:
		return styles.Warning.Render(fmt.Sprintf("%s %s%d", domain.BadgeTextOrigin, domain.BadgeGlyphBehind, s.OriginBehind))
	case domain.DivergenceAhead:
		return styles.Muted.Render(fmt.Sprintf("%s %s%d", domain.BadgeTextOrigin, domain.BadgeGlyphAhead, s.OriginAhead))
	case domain.DivergenceDiverged:
		return styles.DangerText.Render(fmt.Sprintf("%s %s%d %s%d", domain.BadgeTextOrigin, domain.BadgeGlyphAhead, s.OriginAhead, domain.BadgeGlyphBehind, s.OriginBehind))
	default:
		return ""
	}
}

func columnWidths(rows []row) [7]int {
	var widths [7]int
	for _, r := range rows {
		widths[0] = max(widths[0], printableLen(r.branch))
		widths[1] = max(widths[1], printableLen(r.tag))
		widths[2] = max(widths[2], printableLen(r.pr))
		widths[3] = max(widths[3], printableLen(r.services))
		widths[4] = max(widths[4], printableLen(r.ahead))
		widths[5] = max(widths[5], printableLen(r.origin))
		widths[6] = max(widths[6], printableLen(r.status))
	}
	return widths
}

func formatRow(r row, widths [7]int) string {
	return fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %s",
		widths[0]+ansiOverhead(r.branch), r.branch,
		widths[1]+ansiOverhead(r.tag), r.tag,
		widths[2]+ansiOverhead(r.pr), r.pr,
		widths[3]+ansiOverhead(r.services), r.services,
		widths[4]+ansiOverhead(r.ahead), r.ahead,
		widths[5]+ansiOverhead(r.origin), r.origin,
		r.status,
	)
}

// printableLen returns the length of a string without ANSI escape sequences.
func printableLen(s string) int {
	inEscape := false
	n := 0
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}

func ansiOverhead(s string) int {
	return len(s) - printableLen(s)
}

// WriteWorktreeListJSONParams holds inputs for the JSON list serializer.
type WriteWorktreeListJSONParams struct {
	Statuses []domain.WorktreeStatus
	PRInfos  []domain.PRInfo
	Services []domain.JobInfo
}

// WriteWorktreeListJSON writes a JSON array describing each worktree.
func WriteWorktreeListJSON(w io.Writer, params WriteWorktreeListJSONParams) error {
	entries := make([]domain.WorktreeListEntry, 0, len(params.Statuses))
	for _, s := range params.Statuses {
		entries = append(entries, domain.WorktreeListEntry{
			Branch:           s.Branch,
			Path:             s.Path,
			IsParent:         s.IsParent,
			IsDirty:          s.IsDirty,
			RebaseInProgress: s.RebaseInProgress,
			CommitsAhead:     s.CommitsAhead,
			CreatedAt:        s.CreatedAt,
			Origin:           matchOrigin(s),
			PR:               matchPR(s.Branch, params.PRInfos),
			Services:         matchRunningServices(s.Path, params.Services),
		})
	}
	return encodeJSON(w, entries)
}

// matchOrigin projects a worktree's origin divergence into its JSON summary,
// returning nil when the branch has no origin counterpart (DivergenceUnknown).
func matchOrigin(s domain.WorktreeStatus) *domain.WorktreeListOrigin {
	if s.OriginState == domain.DivergenceUnknown {
		return nil
	}
	return &domain.WorktreeListOrigin{
		Ahead:  s.OriginAhead,
		Behind: s.OriginBehind,
		State:  rules.DivergenceStateString(s.OriginState),
	}
}

func matchPR(branch string, prs []domain.PRInfo) *domain.WorktreeListPR {
	for _, pr := range prs {
		if pr.Branch == branch {
			return &domain.WorktreeListPR{Number: pr.Number, URL: pr.URL, State: pr.State}
		}
	}
	return nil
}

func matchRunningServices(worktreePath string, services []domain.JobInfo) []string {
	names := make([]string, 0)
	for _, svc := range services {
		if svc.WorkDir == worktreePath && rules.IsJobUp(svc.Status) {
			names = append(names, svc.Name)
		}
	}
	return names
}

// WriteWorktreeCreateJSON writes the JSON payload for `create`.
func WriteWorktreeCreateJSON(w io.Writer, v any) error {
	return encodeJSON(w, v)
}

// CreateResultParams holds inputs for the framed create conclusion.
type CreateResultParams struct {
	Branch        string
	AlreadyExists bool
	From          string
	EnvStrategy   string
	// EnvNote qualifies the env line with what the port pass did — a count and an
	// offset, resolved by the caller (rules.EnvPortSettlementNote). Empty when the
	// run moved no linked value.
	EnvNote string
	Path    string
	// ExistingBranch reports that an existing local branch was checked out as-is,
	// which retitles the headline and relabels From as the sync parent.
	ExistingBranch bool
	// ReusedNote, set only when ExistingBranch, states how the reused branch
	// relates to origin (behind, diverged, or up to date) — the caller resolves
	// its text and severity (shared.ReusedBranchNote); this only renders it.
	ReusedNote        string
	ReusedNoteWarning bool
	// GoCommand is the ready-to-run jump-in command (e.g. "wtm go feat-x").
	GoCommand string
}

// FormatCreateResult prints the create conclusion: a ✓ headline, an aligned
// summary (from / env / path), then a highlighted `wtm go` step to jump straight
// into the new worktree. A reused branch says so in the headline and labels its
// source "parent", since it was not a start-point. The idempotent already-exists
// case collapses to a single line + the jump-in step. Raw body — the caller's
// frame owns the outer padding.
func FormatCreateResult(w io.Writer, p CreateResultParams) {
	if p.AlreadyExists {
		Unchanged(w, fmt.Sprintf("Worktree %s already exists at %s", p.Branch, p.Path))
		Blank(w)
		NextStep(w, NextStepParams{Command: p.GoCommand})
		return
	}

	headline := fmt.Sprintf("Created worktree %s", p.Branch)
	sourceLabel := domain.CreateRecapLabelFrom
	if p.ExistingBranch {
		headline = fmt.Sprintf(domain.BranchReusedHeadline, p.Branch)
		sourceLabel = domain.CreateRecapLabelParent
	}

	Success(w, headline)
	Blank(w)
	writeAlignedFields(w, []domain.RecapField{
		{Label: sourceLabel, Value: p.From},
		{Label: domain.CreateRecapLabelEnv, Value: withNote(noteParams{Value: p.EnvStrategy, Note: p.EnvNote})},
		{Label: domain.CreateRecapLabelPath, Value: p.Path},
	})
	if p.ReusedNote != "" {
		Blank(w)
		if p.ReusedNoteWarning {
			Warning(w, p.ReusedNote)
		} else {
			Message(w, p.ReusedNote)
		}
	}
	Blank(w)
	NextStep(w, NextStepParams{Command: p.GoCommand})
}

type noteParams struct {
	Value string
	Note  string
}

func withNote(params noteParams) string {
	if params.Note == "" {
		return params.Value
	}
	return params.Value + styles.Muted.Render(domain.EnvRecapNoteSeparator+params.Note)
}

// writeAlignedFields prints indented "label   value" rows with values aligned to a
// common column. Labels are plain ASCII, so byte length equals printable width.
func writeAlignedFields(w io.Writer, fields []domain.RecapField) {
	width := 0
	for _, f := range fields {
		if l := len(f.Label); l > width {
			width = l
		}
	}
	for _, f := range fields {
		pad := strings.Repeat(" ", width-len(f.Label))
		fmt.Fprintf(w, "%s%s%s  %s\n", Indent, styles.Muted.Render(f.Label), pad, f.Value)
	}
}

// WriteWorktreeCleanJSONParams holds inputs for the clean payload.
type WriteWorktreeCleanJSONParams struct {
	Branch        string                  `json:"branch"`
	Path          string                  `json:"path"`
	AlreadyAbsent bool                    `json:"already_absent"`
	Reparented    []domain.ReparentResult `json:"reparented,omitempty"`
	// OrphanedChildren lists children left dangling because reparenting was not
	// authorized (no --reparent-children in non-interactive mode).
	OrphanedChildren []domain.ReparentResult `json:"orphaned_children,omitempty"`
}

// WriteWorktreeCleanJSON writes the JSON payload for `clean`.
func WriteWorktreeCleanJSON(w io.Writer, params WriteWorktreeCleanJSONParams) error {
	return encodeJSON(w, params)
}

// WriteReparentJSONParams holds the reparent payload: the list of worktrees whose
// parent changed, each with its old and new parent. The array shape (mirroring
// clean/prune's `reparented`) fits the command reparenting several worktrees at once.
type WriteReparentJSONParams struct {
	Reparented []domain.ReparentResult `json:"reparented"`
}

// WriteReparentJSON writes the JSON payload for `reparent`.
func WriteReparentJSON(w io.Writer, results []domain.ReparentResult) error {
	return encodeJSON(w, WriteReparentJSONParams{Reparented: results})
}

// encodeJSON writes v as indented JSON to w.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
