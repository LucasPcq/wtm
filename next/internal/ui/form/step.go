package form

import (
	"cmp"

	"github.com/LucasPcq/wtm/next/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	none  = "none"
	trail = " · "
)

type Step interface {
	Label() string
	Description() string
	Standing() Standing
	Open() tea.Cmd
	Field() Field
	Answer() string
	Reset()
}

type Refresher interface {
	Refreshable() bool
	Refreshed()
}

type Standing struct {
	Value  string
	Flag   string
	Origin domain.Origin
	Skip   string
	Erred  bool
}

type mark struct {
	asked bool
	ruled string
	line  string
}

func decided(st Standing, answered bool) mark {
	value := cmp.Or(st.Value, none)
	switch {
	case st.Skip != "":
		return mark{ruled: st.Skip}
	case st.Erred:
		return mark{asked: true}
	case st.Flag != "":
		return mark{line: value + trail + st.Flag}
	case answered:
		return mark{line: value}
	case st.Origin == domain.OriginRemembered:
		return mark{line: value + trail + string(domain.OriginRemembered)}
	}
	return mark{asked: true}
}

func summarized(offered Standing, shown string) string {
	kept := offered.Origin == domain.OriginConfig || offered.Origin == domain.OriginDefault
	if kept && shown == offered.Value {
		return shown + trail + string(offered.Origin)
	}
	return shown
}
