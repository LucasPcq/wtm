package styles

import "github.com/charmbracelet/lipgloss"

var (
	// DashboardPanel frames a dashboard panel. Callers set Width/Height; the
	// border and the title row are what DashboardChromeHeight accounts for.
	DashboardPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1)

	// DashboardPanelTitle heads a panel body.
	DashboardPanelTitle = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	// DashboardWordmark is the product's name in the header bar: the one place
	// the dashboard says what it is, and one of the three uses of the signature
	// accent (the others are DashboardAddButton and DashboardUpgrade).
	DashboardWordmark = lipgloss.NewStyle().Foreground(ColorSignature).Bold(true).Padding(0, 1)

	// DashboardTabActive and DashboardTabInactive render the tab bar entries. The
	// active one is named by weight and by the rule under it, not by a filled
	// block: a background that loud belongs to what the user is acting on.
	DashboardTabActive = lipgloss.NewStyle().Bold(true).Padding(0, 2)

	DashboardTabInactive = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 2)

	// DashboardTabRule underlines the active tab, DashboardRule carries it across
	// the rest of the header.
	DashboardTabRule = lipgloss.NewStyle().Foreground(ColorPrimary)
	DashboardRule    = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardCount is the header's right-hand count.
	DashboardCount = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)

	// DashboardContext renders the header's context line — repo name, base
	// branch, active worktree, fetch staleness. It is structure, not a state,
	// so it stays muted like the rest of the chrome.
	DashboardContext = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardPanelTab* draw the right-hand panel's tabs: the active one in the
	// weight a panel title takes, the idle one muted, and the one a project
	// without a run module cannot reach dimmer still — it is shown, not hidden,
	// so the panel does not change shape between projects.
	DashboardPanelTabActive = DashboardPanelTitle
	DashboardPanelTabIdle   = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardPanelTabOff    = lipgloss.NewStyle().Foreground(ColorMuted).Faint(true)

	// DashboardSectionTitle heads a group of fields inside a panel. It separates
	// two groups, it does not accent one: muted, not the navigation color.
	DashboardSectionTitle = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	// DashboardLabel names a detail field, DashboardValue carries it.
	DashboardLabel = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardValue = lipgloss.NewStyle()

	// DashboardURL is the one full-intensity text of a rowed section: the palette
	// keeps ColorPrimary for navigation, so a job's address is emphasised by
	// everything around it withdrawing to ColorMuted, plus the underline that
	// says "link".
	DashboardURL = lipgloss.NewStyle().Underline(true)

	// DashboardChip renders a non-state element of the vital strip, DashboardChipSep
	// what separates them. The working-tree state has its own, coloured styles: it
	// is the only coloured thing in the strip.
	DashboardChip    = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardChipSep = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardBlockers carries the line of refusals sitting just under the vital
	// strip — why the menu would refuse to delete this worktree.
	DashboardBlockers = lipgloss.NewStyle().Foreground(ColorWarning)

	// DashboardStale mutes a panel body whose data is mid-reload: it does not
	// move, it just says it is not the freshest read anymore.
	DashboardStale = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardBranch renders the detail panel's heading branch name.
	DashboardBranch = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// DashboardEmpty renders a panel's placeholder body.
	DashboardEmpty = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	// DashboardClip carries no styling of its own; it exists so other packages can
	// hard-trim a rendered line to a cell width without instantiating a style.
	DashboardClip = lipgloss.NewStyle()

	// DashboardHelp renders the bottom key hint bar.
	DashboardHelp = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)

	// DashboardHelpBox frames the key and mouse reference. Its border is muted
	// where a modal's is accented: the accent says a box is waiting on an
	// answer, and the reference asks for nothing.
	DashboardHelpBox = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorMuted).
				Padding(0, 1)

	// DashboardHelpKey weights the key over what it does: a reference is read by
	// its key column, so that column carries the weight the description used to.
	DashboardHelpKey = lipgloss.NewStyle().Bold(true)
)

var (
	// DashboardModal frames the box a flow's questions are asked in.
	DashboardModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	// DashboardModalTitle heads the modal, DashboardModalHint closes it with the
	// keys that drive it.
	DashboardModalTitle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DashboardModalHint  = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardRow renders one interactive modal row, DashboardRowFocused the one
	// the keyboard is on.
	DashboardRow        = lipgloss.NewStyle()
	DashboardRowFocused = lipgloss.NewStyle().Foreground(ColorSelectedFg).Background(ColorSelectedBg)

	// DashboardDanger renders what destroys something.
	DashboardDanger = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)

	// DashboardMenu frames the floating context menu, DashboardMenuTitle names
	// the worktree it acts on.
	DashboardMenu = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	DashboardMenuTitle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// DashboardAddButton is the header's primary call to action, and one of the
	// three uses of the signature accent: it stands alone in a title row, with no
	// focus ring around it to say what it is.
	DashboardAddButton = lipgloss.NewStyle().
				Foreground(ColorBadgeFg).
				Background(ColorSignature).
				Bold(true).
				Padding(0, 3)

	// DashboardUpgrade is the "run wtm upgrade" segment of the header's context
	// row — the third use of the signature accent, and the reason it is not muted
	// like the facts it sits among: it is the only thing on that row to act on.
	DashboardUpgrade = lipgloss.NewStyle().Foreground(ColorSignature).Bold(true)

	// DashboardHeaderButton is a secondary header action: muted, so the filled
	// call to action is the bar's only accent — two equal-weight calls would
	// have said neither of them.
	DashboardHeaderButton = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true).Padding(0, 2)

	// DashboardDisabled renders what cannot be activated yet.
	DashboardDisabled = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	// DashboardRowSelected tints the whole selected worktree row. Render plain
	// (ANSI-free) text through it so the background fills every cell.
	DashboardRowSelected = lipgloss.NewStyle().
				Background(ColorRowTint).
				Foreground(ColorSelectedFg)

	// DashboardRecapValue weights the value of a recap field over its label.
	DashboardRecapValue = lipgloss.NewStyle().Bold(true)

	// DashboardTreeGutter draws the connector run down the Tree tab, quieter than
	// anything it connects; DashboardTreeVirtual renders a node standing in for a
	// branch with no worktree, and DashboardTreeWarn its warning badges.
	DashboardTreeGutter  = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardTreeVirtual = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	// DashboardTreeRoot weights the repository's own worktree, the one every tree
	// hangs from. DashboardTreeNode is muted: a bullet is structure, and it is
	// the node's own state — when it has one — that colors it, not the tree.
	DashboardTreeRoot = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DashboardTreeNode = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardTreeWarn = lipgloss.NewStyle().Foreground(ColorWarning)

	// DashboardRowBar is the accent bar down the left of the selected row,
	// DashboardRowName the worktree's own name — the heaviest thing in the list —
	// and DashboardRowMeta the quieter line under it.
	DashboardRowBar  = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DashboardRowName = lipgloss.NewStyle().Bold(true)
	DashboardRowMeta = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardRowFlashBright is a just-created row's opening beat: brighter
	// than the ordinary selected tint, before it settles into
	// DashboardRowSelected — see rules.FlashLit.
	DashboardRowFlashBright = lipgloss.NewStyle().
				Background(ColorFlashTint).
				Foreground(ColorSelectedFg)
)
