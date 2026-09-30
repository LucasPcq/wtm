package styles

import "github.com/charmbracelet/lipgloss"

var (
	// RunViewPane frames a job's terminal emulator. No padding: every column
	// inside the border is one the job wrote to, and one the emulator was sized
	// for.
	RunViewPane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted)

	// RunViewPaneFocused frames the same pane while the keyboard belongs to the
	// job inside it. The accent on the border is the whole indicator: nothing
	// else on screen changes, because nothing else changed.
	RunViewPaneFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary)

	// RunViewSidebar frames the job list beside it.
	RunViewSidebar = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1)

	// RunViewJobSelected marks the job whose pane is on screen.
	RunViewJobSelected = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// RunViewWorktreeHeading and RunViewSharedHeading head the groups of the job
	// list and of the reach block. A worktree reads in the foreground, bold, so
	// it outranks the jobs under it; the shared services take the colour of
	// their mark, since they are the one group no worktree owns.
	RunViewWorktreeHeading = lipgloss.NewStyle().Bold(true)
	RunViewSharedHeading   = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
)
