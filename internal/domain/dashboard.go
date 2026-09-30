package domain

// Rect is a screen region in terminal cells, top-left origin, zero-based.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// DashboardLayout is the resolved geometry of one dashboard frame. Every panel
// is placed here so the renderer only draws and the hit-testing has a single
// reference to agree with.
type DashboardLayout struct {
	Narrow bool
	// HeaderTall reports which header rules.ComputeDashboardLayout chose for
	// this frame: the six-row signature block when true, the compact
	// three-row header otherwise. The renderer reads it rather than
	// re-deriving the choice from Tabs.Height against a threshold of its own.
	HeaderTall bool

	Tabs   Rect
	List   Rect
	Detail Rect
	Output Rect
	Help   Rect

	ListVisible   bool
	DetailVisible bool

	// ListRows is how many worktree rows fit in the list body, TreeRows how many
	// tree nodes fit in the same space (one line each), OutputLines how many lines
	// fit in the output body (0 when it is folded).
	ListRows int
	TreeRows int
	// ServicesRows is how many lines the Services tab draws. One per row, and
	// therefore not TreeRows: a tree node takes two lines, the spacer under it
	// included.
	ServicesRows int
	OutputLines  int
}

// HelpEntry is one row of the key and mouse reference: what to press, and what
// pressing it does.
type HelpEntry struct {
	Keys string
	Text string
}

// HelpSection groups entries under a title of its own.
type HelpSection struct {
	Title   string
	Entries []HelpEntry
}

// HelpBand is one row of sections in the reference overlay: a single section in
// the narrow layout, two side by side in the wide one. Sections in a band are
// drawn to the same height so the next band starts on one line in every column.
type HelpBand []HelpSection

// HelpLayout is the resolved geometry of the reference overlay. Nothing here is
// styled or rendered: the columns are already chosen, so the renderer only
// stacks and pads.
type HelpLayout struct {
	Bands []HelpBand
	// KeyWidth, TextWidth and ColumnWidth are indexed by column, so a column is
	// sized on what it actually carries rather than on the widest row of the
	// whole overlay.
	KeyWidth    []int
	TextWidth   []int
	ColumnWidth []int
	// Inner is the text width inside the box's border and padding.
	Inner int
	// ContentRows is what the bands render to, BodyRows how many of those the
	// screen leaves room for, and Scrollable reports that the two differ.
	ContentRows int
	BodyRows    int
	Scrollable  bool
}
