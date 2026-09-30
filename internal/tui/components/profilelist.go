package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	profileNameCharLimit = 40
	profileNameWidth     = 32
	// minProfileJobsWidth keeps the wrapped job list readable on a narrow
	// terminal rather than folding it to one job per line.
	minProfileJobsWidth = 20
	// noMark is the cursor position no row is marked at.
	noMark = -1
)

// ProfileListModel edits the split `run up` will offer. The proposal it starts
// from is a guess about an intention — grouping packages into apps is something
// only the user knows — so every operation here exists to correct it.
type ProfileListModel struct {
	profiles []domain.ProfileConfig
	cursor   int
	offset   int
	mark     int
	width    int
	height   int
	title    string
	desc     string
	done     bool
	aborted  bool

	naming    bool
	namingNew bool
	input     textinput.Model
	err       string
}

type NewProfileListParams struct {
	Title       string
	Description string
	Profiles    []domain.ProfileConfig
}

func NewProfileList(params NewProfileListParams) ProfileListModel {
	// Copied, never aliased: the proposal it starts from can be the loaded
	// config's own slice, and renaming a profile through it edited run.toml —
	// a rename the user then escaped out of survived.
	profiles := make([]domain.ProfileConfig, len(params.Profiles))
	copy(profiles, params.Profiles)

	return ProfileListModel{
		profiles: profiles,
		mark:     noMark,
		title:    params.Title,
		desc:     params.Description,
		width:    80,
	}
}

func (m ProfileListModel) Profiles() []domain.ProfileConfig { return m.profiles }
func (m ProfileListModel) Done() bool                       { return m.done }
func (m ProfileListModel) Aborted() bool                    { return m.aborted }
func (m ProfileListModel) Init() tea.Cmd                    { return nil }

func (m *ProfileListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m ProfileListModel) doneRow() int { return len(m.profiles) }

func (m ProfileListModel) Update(msg tea.Msg) (ProfileListModel, tea.Cmd) {
	if m.naming {
		return m.updateNaming(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < m.doneRow() {
			m.cursor++
		}
	case domain.ProfileListKeyRemove:
		return m.remove(), nil
	case domain.ProfileListKeyMerge:
		return m.mergeStep(), nil
	case domain.ProfileListKeyRename:
		if m.cursor < m.doneRow() {
			return m.startNaming(false), nil
		}
	case domain.ProfileListKeyNew:
		return m.startNaming(true), nil
	case "enter":
		if m.cursor == m.doneRow() {
			m.done = true
		}
	case "esc":
		if m.mark != noMark {
			m.mark = noMark
			return m, nil
		}
		m.aborted = true
	}

	return m.scrolled(), nil
}

func (m ProfileListModel) remove() ProfileListModel {
	if m.cursor >= m.doneRow() {
		return m
	}
	m.profiles = append(append([]domain.ProfileConfig{}, m.profiles[:m.cursor]...), m.profiles[m.cursor+1:]...)
	m.mark = noMark
	if m.cursor > 0 && m.cursor >= len(m.profiles) {
		m.cursor = len(m.profiles)
	}
	return m.ensureDefault()
}

// mergeStep is both halves of a merge: the first press marks the row the jobs
// will land in, the second folds the row under the cursor into it.
func (m ProfileListModel) mergeStep() ProfileListModel {
	if m.cursor >= m.doneRow() {
		return m
	}
	if m.mark == noMark {
		m.mark = m.cursor
		return m
	}
	if m.mark == m.cursor {
		m.mark = noMark
		return m
	}

	target, source := m.mark, m.cursor
	merged := m.profiles[target]
	merged.Jobs = unionJobs(merged.Jobs, m.profiles[source].Jobs)
	merged.Default = merged.Default || m.profiles[source].Default

	kept := make([]domain.ProfileConfig, 0, len(m.profiles)-1)
	for i, profile := range m.profiles {
		switch i {
		case target:
			kept = append(kept, merged)
		case source:
		default:
			kept = append(kept, profile)
		}
	}

	m.profiles = kept
	m.mark = noMark
	if m.cursor >= len(m.profiles) {
		m.cursor = len(m.profiles)
	}
	return m.ensureDefault()
}

// unionJobs keeps the target's order and appends what the source adds. A job
// both profiles carry — the shared infrastructure — must not be started twice.
func unionJobs(target, source []string) []string {
	seen := make(map[string]bool, len(target))
	for _, job := range target {
		seen[job] = true
	}
	merged := append([]string{}, target...)
	for _, job := range source {
		if !seen[job] {
			seen[job] = true
			merged = append(merged, job)
		}
	}
	return merged
}

// ensureDefault keeps exactly one profile marked: the picker pre-selects it,
// and losing it would leave the picker without a starting point.
func (m ProfileListModel) ensureDefault() ProfileListModel {
	if len(m.profiles) == 0 {
		return m
	}
	for _, profile := range m.profiles {
		if profile.Default {
			return m
		}
	}
	m.profiles[0].Default = true
	return m
}

func (m ProfileListModel) startNaming(creating bool) ProfileListModel {
	input := textinput.New()
	input.CharLimit = profileNameCharLimit
	input.Width = profileNameWidth
	if !creating {
		input.Placeholder = m.profiles[m.cursor].Name
	}
	input.Focus()

	m.input = input
	m.naming = true
	m.namingNew = creating
	m.err = ""
	return m
}

func (m ProfileListModel) updateNaming(msg tea.Msg) (ProfileListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "enter":
		return m.saveName(), nil
	case "esc":
		m.naming = false
		m.err = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m ProfileListModel) saveName() ProfileListModel {
	name := strings.TrimSpace(m.input.Value())
	if name == "" && !m.namingNew {
		m.naming = false
		return m
	}
	if name == "" {
		m.err = domain.ProfileListNameRequired
		return m
	}
	for i, profile := range m.profiles {
		if profile.Name == name && (m.namingNew || i != m.cursor) {
			m.err = fmt.Sprintf(domain.ProfileListNameTakenFmt, name)
			return m
		}
	}

	if m.namingNew {
		m.profiles = append(m.profiles, domain.ProfileConfig{Name: name})
		m.cursor = len(m.profiles) - 1
	} else {
		m.profiles[m.cursor].Name = name
	}

	m.naming = false
	m.err = ""
	return m.ensureDefault()
}

func (m ProfileListModel) View() string {
	body, _ := windowBody(m.window())
	return body
}

// window lays the list out one line per row and, alongside it, the span the
// cursor occupies — its row plus the jobs that unfold under it, so a scroll
// never separates a profile from the services it holds.
func (m ProfileListModel) window() bodyWindowParams {
	rows := make([]string, 0, len(m.profiles)+1)
	top, bottom := 0, 0
	for i, profile := range m.profiles {
		selected := i == m.cursor
		label := profileHeadLabel(profile)
		if m.naming && !m.namingNew && selected {
			label = m.input.View()
		}
		if i == m.mark {
			label = domain.ProfileListMarkPrefix + label
		}
		if selected {
			top = len(rows)
		}
		rows = append(rows, m.renderRow(label, selected))
		if !selected {
			continue
		}
		for _, line := range m.jobLines(profile) {
			rows = append(rows, styles.Muted.Render(domain.ProfileListJobsIndent+line))
		}
		bottom = len(rows) - 1
	}
	if m.naming && m.namingNew {
		rows = append(rows, m.renderRow(m.input.View(), true))
	}
	if m.cursor == m.doneRow() {
		top, bottom = len(rows), len(rows)
	}
	rows = append(rows, m.renderRow(domain.WizardDoneRow, m.cursor == m.doneRow()))

	extras := ""
	if m.err != "" {
		extras = "\n\n" + errorBanner(m.err)
	}
	return bodyWindowParams{
		Rows: rows, Offset: m.offset, Height: m.height,
		Top: top, Bottom: bottom, Extras: extras,
	}
}

// scrolled settles where the window sits after the cursor moved, so the next
// render scrolls from there rather than snapping back to the top of the list.
func (m ProfileListModel) scrolled() ProfileListModel {
	_, m.offset = windowBody(m.window())
	return m
}

// helpHint is the wizard help bar for this step. Marking a row changes what the
// next keypress does, so the bar has to say so — the second half of a merge is
// not something to guess.
func (m ProfileListModel) helpActions() []string {
	return []string{domain.HelpRename, domain.HelpMerge, domain.HelpDelete, domain.HelpNew}
}

func (m ProfileListModel) helpModal() string {
	if m.naming {
		return domain.ProfileListNamingHelp
	}
	if m.mark != noMark {
		return fmt.Sprintf(domain.ProfileListMergeHint, m.profiles[m.mark].Name)
	}
	return ""
}

// profileHeadLabel is the row itself: the name and what marks it, never the
// jobs. A profile can hold a dozen of them, and nothing here wrapped or
// truncated, so the row ran past the terminal and took the highlight with it
// (LUC-208). The jobs are shown under the cursor instead, by jobLines.
func profileHeadLabel(profile domain.ProfileConfig) string {
	if profile.Default {
		return profile.Name + domain.ProfileListDefaultSuffix
	}
	return profile.Name
}

// jobLines wraps a profile's jobs under its row, so the whole list is readable
// by moving onto it. Only the row under the cursor gets them: every profile
// unfolded at once is the same wall of text the single line was.
func (m ProfileListModel) jobLines(profile domain.ProfileConfig) []string {
	if len(profile.Jobs) == 0 {
		return nil
	}

	width := max(m.width-len(domain.ProfileListJobsIndent), minProfileJobsWidth)
	var lines []string
	current := ""
	for _, job := range profile.Jobs {
		candidate := job
		if current != "" {
			candidate = current + domain.ProfileListJobSep + job
		}
		if current != "" && len(candidate) > width {
			lines = append(lines, current+domain.ProfileListJobSep)
			current = job
			continue
		}
		current = candidate
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func (m ProfileListModel) renderRow(label string, selected bool) string {
	if selected {
		line := truncateLabel("▸ "+label, m.width)
		if pad := m.width - PrintableWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		return styles.ListItemSelected.Render(line)
	}
	return styles.ListItemNormal.Render(truncateLabel(styles.Indent+label, m.width))
}

// truncateLabel keeps a row inside the terminal. A label longer than the screen
// is wrapped by the terminal itself, which breaks the row highlight across two
// lines and pushes the rest of the list down.
func truncateLabel(label string, width int) string {
	if width <= 0 || PrintableWidth(label) <= width {
		return label
	}
	kept := make([]rune, 0, width)
	for _, r := range label {
		if len(kept)+len([]rune(domain.ProfileListEllipsis)) >= width {
			break
		}
		kept = append(kept, r)
	}
	return string(kept) + domain.ProfileListEllipsis
}
