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

type settled struct {
	Standing Standing
	Answered bool
}

func decided(s settled) mark {
	st := s.Standing
	value := cmp.Or(st.Value, none)
	switch {
	case st.Skip != "":
		return mark{ruled: st.Skip}
	case st.Erred:
		return mark{asked: true}
	case st.Flag != "":
		return mark{line: value + trail + st.Flag}
	case s.Answered:
		return mark{line: value}
	case st.Origin == domain.OriginRemembered:
		return mark{line: value + trail + string(domain.OriginRemembered)}
	}
	return mark{asked: true}
}

type shown struct {
	Offered Standing
	Shown   string
}

func summarized(s shown) string {
	kept := s.Offered.Origin == domain.OriginConfig || s.Offered.Origin == domain.OriginDefault
	if kept && s.Shown == s.Offered.Value {
		return s.Shown + trail + string(s.Offered.Origin)
	}
	return s.Shown
}
