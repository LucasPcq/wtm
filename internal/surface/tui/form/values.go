package form

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

func valueOf(model any) kernel.Value {
	switch answered := model.(type) {
	case components.TextInputModel:
		return kernel.Text(answered.Value())
	case components.ConfirmModel:
		return kernel.Bool(answered.Confirmed())
	case components.SelectListModel:
		return kernel.Text(answered.Value())
	case components.MultiSelectModel:
		return kernel.List(answered.Values())
	case components.ReorderListModel:
		return kernel.List(answered.Values())
	case components.TextListModel:
		return kernel.List(answered.Values())
	case components.DecisionListModel:
		return kernel.Decisions(answered.Values())
	}
	return nil
}

func (s *session) summary(i int, model any) string {
	value := valueOf(model)
	decided := s.decided[i]
	kept := decided.origin == kernel.OriginDefault || decided.origin == kernel.OriginConfig
	if !kept || !reflect.DeepEqual(value, decided.offered) {
		return valueText(value)
	}
	return trailOf(valueText(value), string(decided.origin))
}

func valueText(value kernel.Value) string {
	switch shown := value.(type) {
	case kernel.Text:
		return string(shown)
	case kernel.Bool:
		if shown {
			return yesText
		}
		return noText
	case kernel.List:
		if len(shown) == 0 {
			return domain.SummaryNone
		}
		return strings.Join(shown, listSeparator)
	case kernel.Decisions:
		pairs := make([]string, 0, len(shown))
		for _, key := range slices.Sorted(maps.Keys(shown)) {
			pairs = append(pairs, fmt.Sprintf(pairFmt, key, shown[key]))
		}
		if len(pairs) == 0 {
			return domain.SummaryNone
		}
		return strings.Join(pairs, listSeparator)
	}
	return ""
}

func trailOf(value, origin string) string {
	return fmt.Sprintf(trailFmt, value, origin)
}

func textOf(value kernel.Value) string {
	shown, _ := value.(kernel.Text)
	return string(shown)
}

func list(value kernel.Value) []string {
	shown, _ := value.(kernel.List)
	return shown
}
