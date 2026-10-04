package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type finding struct {
	pos  token.Position
	rule string
	msg  string
	// legacy marks what predates its rule and is tracked in the code rather than
	// in .archlint-migrating, keyed so its sites collapse to one line.
	legacy string
	// overBudget is a legacy key with more sites than its recorded count.
	overBudget bool
	// note reports without being a site: a count that can be lowered.
	note bool
}

// migrating is what predates a rule and is tracked out rather than fixed on the
// spot. It reports without failing, and the list may only ever shrink: each
// entry records how many sites it covers, a site beyond that count fails, and a
// count higher than needed is reported to be lowered.
type migratingList struct {
	entries []migratingEntry
}

type migratingEntry struct {
	line    string
	rule    string
	pattern *regexp.Regexp
	budget  int
}

func (m migratingList) match(f finding) int {
	for index, entry := range m.entries {
		if entry.rule == f.rule && entry.pattern.MatchString(filepath.ToSlash(f.pos.Filename)) {
			return index
		}
	}
	return -1
}

func loadMigrating() (migratingList, error) {
	data, err := os.ReadFile(".archlint-migrating")
	if os.IsNotExist(err) {
		return migratingList{}, nil
	}
	if err != nil {
		return migratingList{}, err
	}
	return parseMigrating(string(data))
}

func parseMigrating(data string) (migratingList, error) {
	var list migratingList
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return list, fmt.Errorf(".archlint-migrating: %q is not `<rule> <path regex> <sites>`", line)
		}
		budget, err := strconv.Atoi(fields[2])
		if err != nil || budget < 1 {
			return list, fmt.Errorf(".archlint-migrating: %q: the site count must be a positive number", line)
		}
		compiled, err := regexp.Compile(fields[1])
		if err != nil {
			return list, fmt.Errorf(".archlint-migrating: %w", err)
		}
		list.entries = append(list.entries, migratingEntry{line: line, rule: fields[0], pattern: compiled, budget: budget})
	}
	return list, nil
}

type judgeParams struct {
	Findings  []finding
	Migrating migratingList
	Warned    map[string]bool
}

type verdict struct {
	lines  []string
	notes  []string
	failed bool
}

func judge(params judgeParams) verdict {
	var result verdict
	used := make([]int, len(params.Migrating.entries))
	for _, f := range params.Findings {
		if f.note {
			result.notes = append(result.notes, fmt.Sprintf("[%s] %s", f.rule, f.msg))
			continue
		}
		tag := tagOf(tagParams{Finding: f, Migrating: params.Migrating, Used: used, Warned: params.Warned})
		if tag == "" || tag == tagOverBudget {
			result.failed = true
		}
		result.lines = append(result.lines, fmt.Sprintf("%s:%d:%d: [%s]%s %s", f.pos.Filename, f.pos.Line, f.pos.Column, f.rule, tag, f.msg))
	}
	for index, entry := range params.Migrating.entries {
		switch {
		case used[index] == 0:
			result.notes = append(result.notes, fmt.Sprintf(".archlint-migrating: %q covers nothing any more — remove it", entry.line))
		case used[index] < entry.budget:
			result.notes = append(result.notes, fmt.Sprintf(".archlint-migrating: %q covers %d site(s) — lower it to %d", entry.line, used[index], used[index]))
		}
	}
	return result
}

const (
	tagWarning    = " (warning)"
	tagMigrating  = " (migrating)"
	tagOverBudget = " (over its recorded count — the list may only shrink)"
)

type tagParams struct {
	Finding   finding
	Migrating migratingList
	Used      []int
	Warned    map[string]bool
}

func tagOf(params tagParams) string {
	f := params.Finding
	if params.Warned[f.rule] {
		return tagWarning
	}
	if f.overBudget {
		return tagOverBudget
	}
	if f.legacy != "" {
		return tagMigrating
	}
	index := params.Migrating.match(f)
	if index < 0 {
		return ""
	}
	params.Used[index]++
	if params.Used[index] > params.Migrating.entries[index].budget {
		return tagOverBudget
	}
	return tagMigrating
}

type collapseLegacyParams struct {
	Findings []finding
	// Budgets is the recorded site count of each legacy key.
	Budgets map[string]int
}

// collapseLegacy keeps the first site of each legacy key and counts the rest,
// so a tracked debt is one line of the report rather than a page of it.
func collapseLegacy(params collapseLegacyParams) []finding {
	findings := params.Findings
	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].pos.String() < findings[j].pos.String()
	})
	counts := map[string]int{}
	for _, f := range findings {
		if f.legacy != "" {
			counts[f.rule+f.legacy]++
		}
	}
	var kept []finding
	done := map[string]bool{}
	for _, f := range findings {
		key := f.rule + f.legacy
		if f.legacy == "" {
			kept = append(kept, f)
			continue
		}
		if done[key] {
			continue
		}
		done[key] = true
		f.msg = fmt.Sprintf("%s (%d sites)", f.msg, counts[key])
		budget := params.Budgets[f.legacy]
		f.overBudget = counts[key] > budget
		if counts[key] < budget {
			f.msg = fmt.Sprintf("%s — its recorded count is %d, lower it to %d", f.msg, budget, counts[key])
		}
		kept = append(kept, f)
	}
	for _, legacy := range sortedKeys(params.Budgets) {
		if !seenLegacy(kept, legacy) {
			kept = append(kept, finding{rule: "fontcover", note: true, msg: fmt.Sprintf("%q has no site left — remove it from fontLegacy", legacy)})
		}
	}
	return kept
}

func seenLegacy(findings []finding, legacy string) bool {
	for _, f := range findings {
		if f.legacy == legacy {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
