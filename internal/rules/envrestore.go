package rules

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

// OwnedEnvKeyRefs are every .env key wtm writes into an isolated worktree:
// the [[env_port]] links, the [[env]] values and the identity keys of the files
// a compose stack reads.
func OwnedEnvKeyRefs(params OwnedEnvTargetsParams) []domain.EnvKeyRef {
	var refs []domain.EnvKeyRef
	add := func(ref domain.EnvKeyRef) {
		if !slices.Contains(refs, ref) {
			refs = append(refs, ref)
		}
	}
	for _, link := range params.Config.EnvPorts {
		add(domain.EnvKeyRef{File: link.File, Key: link.Key})
	}
	for _, link := range params.Config.EnvValues {
		add(domain.EnvKeyRef{File: link.File, Key: link.Key})
	}
	for _, target := range OwnedEnvTargets(params) {
		for _, key := range domain.WtmOwnedEnvKeys {
			add(domain.EnvKeyRef{File: target, Key: key})
		}
	}
	return refs
}

// EnvPortBasesByKey are the base ports each [[env_port]] key follows, in the
// order run.toml declares them.
func EnvPortBasesByKey(cfg domain.RunConfig) map[domain.EnvKeyRef][]int {
	bases := EnvPortBases(cfg)
	byKey := map[domain.EnvKeyRef][]int{}
	for _, link := range cfg.EnvPorts {
		base, found := EnvPortBaseFor(bases, link)
		if !found {
			continue
		}
		ref := domain.EnvKeyRef{File: link.File, Key: link.Key}
		if !slices.Contains(byKey[ref], base) {
			byKey[ref] = append(byKey[ref], base)
		}
	}
	return byKey
}

type RestoreOwnedEnvParams struct {
	File   string
	Child  []domain.EnvLine
	Source []domain.EnvLine
	Keys   []string
	// PortBases are the base ports run.toml declares for each port-linked key.
	PortBases map[string][]int
}

// RestoreOwnedEnv puts each owned key the child holds back to the source's
// value, and drops the ones the source does not have. A key the child lacks is
// left to the reconciliation, which adds it from the source like any other.
func RestoreOwnedEnv(params RestoreOwnedEnvParams) ([]domain.EnvLine, []domain.EnvRestoredEntry) {
	source := pairsByKey(params.Source)

	var entries []domain.EnvRestoredEntry
	out := make([]domain.EnvLine, 0, len(params.Child))
	for _, line := range params.Child {
		if line.Kind != domain.EnvLinePair || !slices.Contains(params.Keys, line.Key) {
			out = append(out, line)
			continue
		}
		from, present := source[line.Key]
		if !present {
			entries = append(entries, domain.EnvRestoredEntry{File: params.File, Key: line.Key, From: line.Value, Removed: true})
			continue
		}
		if from.Value != line.Value {
			entry := domain.EnvRestoredEntry{File: params.File, Key: line.Key, From: line.Value, To: from.Value}
			if onlyBasePortsDiffer(onlyBasePortsDifferParams{Child: line.Value, Source: from.Value, Bases: params.PortBases[line.Key]}) {
				entry.Ports = params.PortBases[line.Key]
			}
			entries = append(entries, entry)
		}
		out = append(out, WithEnvValue(line, from.Value))
	}
	return out, entries
}

type EnvRestoredRowsParams struct {
	Entries    []domain.EnvRestoredEntry
	File       string
	ShowValues bool
}

type onlyBasePortsDifferParams struct {
	Child  string
	Source string
	Bases  []int
}

// onlyBasePortsDiffer says the restore only moves ports back: the two values
// differ by numbers alone, each one the source holds a declared base port. A
// row then names those ports; any other difference — a password edited in the
// worktree — makes it say the whole value goes back, which is what happens.
// It answers a yes or no and prints nothing of either value.
func onlyBasePortsDiffer(params onlyBasePortsDifferParams) bool {
	child, source := digitRuns(params.Child), digitRuns(params.Source)
	if len(params.Bases) == 0 || len(child) != len(source) {
		return false
	}
	for i := range child {
		if child[i] == source[i] {
			continue
		}
		port, err := strconv.Atoi(source[i])
		if !isDigitRun(child[i]) || err != nil || !slices.Contains(params.Bases, port) {
			return false
		}
	}
	return true
}

// digitRuns cuts a value into its runs of digits and the text between them.
func digitRuns(value string) []string {
	var runs []string
	start := 0
	for i := 1; i <= len(value); i++ {
		if i == len(value) || isDigitByte(value[i]) != isDigitByte(value[start]) {
			runs = append(runs, value[start:i])
			start = i
		}
	}
	return runs
}

func isDigitRun(run string) bool {
	return run != "" && isDigitByte(run[0])
}

func isDigitByte(c byte) bool {
	return c >= '0' && c <= '9'
}

// EnvRestoredRows renders one file's restored values as aligned rows. Without
// --show-values a row names only the ports run.toml declares for the key: the
// value put back is the source's, the user's.
func EnvRestoredRows(params EnvRestoredRowsParams) []string {
	var mine []domain.EnvRestoredEntry
	width := 0
	for _, entry := range params.Entries {
		if entry.File == params.File {
			mine = append(mine, entry)
			width = max(width, len(entry.Key))
		}
	}

	rows := make([]string, 0, len(mine))
	for _, entry := range mine {
		rows = append(rows, pad(entry.Key, width)+domain.EnvKeyRowGap+restoredDetail(restoredDetailParams{Entry: entry, ShowValues: params.ShowValues}))
	}
	return rows
}

type restoredDetailParams struct {
	Entry      domain.EnvRestoredEntry
	ShowValues bool
}

func restoredDetail(params restoredDetailParams) string {
	entry := params.Entry
	if params.ShowValues {
		if entry.Removed {
			return fmt.Sprintf(domain.EnvDetailRestoredRemovedFmt, EnvQuote(entry.From))
		}
		return fmt.Sprintf(domain.EnvDetailRestoredFmt, EnvQuote(entry.To), EnvQuote(entry.From))
	}
	if entry.Removed {
		return domain.EnvDetailRestoredRemoved
	}
	if len(entry.Ports) == 0 {
		return domain.EnvDetailRestoredValue
	}
	return fmt.Sprintf(domain.EnvDetailRestoredPortsFmt, EnvBasePorts(entry.Ports))
}
