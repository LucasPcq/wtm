package styles

type Style struct{ faint bool }

func (s Style) Render(text ...string) string { return "" }

var (
	Muted          = Style{faint: true}
	Accent         = Style{}
	BadgeOK        = Style{}
	DashboardTitle = Style{}
)
