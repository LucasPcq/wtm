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

// A Step is one question of a command's form, implemented by the command.
// The wizard asks it only when its [Standing] leaves it open, and shows it in the trail once settled.
type Step interface {
	Label() string       // the step's name in the breadcrumb and the trail
	Description() string // what the answer decides, shown under the breadcrumb
	// Standing returns where the step's answer stands on the facts observed last.
	Standing() Standing
	// Open builds the step's field from the facts observed last, before the step is asked, and returns its first command.
	Open() tea.Cmd
	Field() Field // the field Open built
	// Answer records the field's answer into the command's request and returns it as the trail shows it.
	Answer() string
	// Reset forgets the step's answer when the user goes back before it.
	Reset()
}

// A Refresher is a [Step] whose facts the user may read again with r while it is asked.
type Refresher interface {
	Refreshable() bool // reports whether r refreshes now, not while the field takes typed text for instance
	Refreshed()        // rebuilds the field from the facts just read again
}

// Standing is where a step's answer stands before the user is asked.
type Standing struct {
	Value  string        // the answer the step holds
	Given  bool          // reports whether the command line set Value, by a flag or an argument: the step is not asked
	Flag   string        // the flag that set Value, if one did, named in the trail
	Origin domain.Origin // where Value comes from; a remembered answer is not asked
	Skip   string        // why the step does not apply, if it does not: the step is not asked
	Erred  bool          // reports whether Value is refused, so the step is asked even when a flag set it
}

// A mark is how the wizard settled a step: asked, ruled out with a reason, or settled with the line the trail shows.
type mark struct {
	asked bool
	ruled string
	line  string
}

// decided returns how a step standing at st is settled; answered reports whether the user answered it in the form a stale plan restarted ([Wizard.Again]).
func decided(st Standing, answered bool) mark {
	value := cmp.Or(st.Value, none)
	switch {
	case st.Skip != "":
		return mark{ruled: st.Skip}
	case st.Erred:
		return mark{asked: true}
	case st.Given && st.Flag != "":
		return mark{line: value + trail + st.Flag}
	case st.Given:
		return mark{line: value}
	case answered:
		return mark{line: value}
	case st.Origin == domain.OriginRemembered:
		return mark{line: value + trail + string(domain.OriginRemembered)}
	}
	return mark{asked: true}
}
