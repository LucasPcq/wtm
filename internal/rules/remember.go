package rules

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RememberableValues is every answer a memory id may hold. A destructive option
// has no place here: nothing outside this table is ever remembered.
func RememberableValues(id string) ([]string, bool) {
	switch id {
	case domain.RememberEnvStrategy:
		return []string{string(domain.EnvStrategyExample), string(domain.EnvStrategyMain), string(domain.EnvStrategyParent)}, true
	case domain.RememberIsolation:
		return []string{string(domain.IsolationIsolated), string(domain.IsolationVerbatim)}, true
	case domain.RememberSourceUpdate:
		return []string{domain.SourceUpdateFastForward, domain.SourceUpdateKeep}, true
	}
	return nil, false
}

func rememberIDs() []string {
	return []string{domain.RememberEnvStrategy, domain.RememberIsolation, domain.RememberSourceUpdate}
}

// ValidateRemembered refuses an id no question remembers under and a value its
// question never offers, at load time rather than when a wizard skips on it.
func ValidateRemembered(remembered map[string]string) error {
	ids := make([]string, 0, len(remembered))
	for id := range remembered {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		allowed, known := RememberableValues(id)
		if !known {
			return fmt.Errorf(domain.RememberedUnknownFmt, id, strings.Join(rememberIDs(), ", "))
		}
		if !slices.Contains(allowed, remembered[id]) {
			return fmt.Errorf(domain.RememberedInvalidFmt, id, remembered[id], strings.Join(allowed, ", "))
		}
	}
	return nil
}

type RememberedChange struct {
	Remember map[string]string
	Forget   []string
}

// ApplyRemembered is the memory once a session settled what to keep and what to
// forget; nil when nothing is left, so the section disappears from the file.
func ApplyRemembered(current map[string]string, change RememberedChange) map[string]string {
	next := make(map[string]string, len(current)+len(change.Remember))
	for id, value := range current {
		next[id] = value
	}
	for _, id := range change.Forget {
		delete(next, id)
	}
	for id, value := range change.Remember {
		next[id] = value
	}
	if len(next) == 0 {
		return nil
	}
	return next
}
