package runenv

import (
	"fmt"
	"sort"
	"strings"
)

const (
	noteSeparator = " · "
	fileRuleFmt   = "── %s "
	ruleRune      = "─"
)

func SettlementNote(plan Redacted) string {
	var parts []string
	if shifted := len(rewrites(Plan(plan))); shifted > 0 {
		parts = append(parts, fmt.Sprintf("%d %s settled (offset +%d)", shifted, plural(pluralParams{Count: shifted, One: "port", Many: "ports"}), plan.Offset))
	}
	if written := changedOwned(plan.Owned); written > 0 {
		parts = append(parts, fmt.Sprintf("%d owned %s written", written, plural(pluralParams{Count: written, One: "value", Many: "values"})))
	}
	return strings.Join(parts, noteSeparator)
}

func AnomalyLines(plan Redacted) []string {
	entries := anomalies(Plan(plan))
	if len(entries) == 0 {
		return nil
	}
	keyWidth, portWidth := 0, 0
	for _, entry := range entries {
		keyWidth = max(keyWidth, len(entry.Key))
		portWidth = max(portWidth, len(entry.Port))
	}
	rows := make(map[fileKey]string, len(entries))
	widest := 0
	for _, entry := range entries {
		row := strings.TrimRight(fmt.Sprintf("%s  %s  %s", pad(entry.Key, keyWidth), pad(entry.Port, portWidth), anomalyReason(entry)), " ")
		rows[entryKey(entry)] = row
		widest = max(widest, len([]rune(row)))
	}
	var lines []string
	for i, file := range filesOf(entries) {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fileRule(fileRuleParams{File: file, Width: widest}))
		for _, entry := range entries {
			if entry.File == file {
				lines = append(lines, rows[entryKey(entry)])
			}
		}
	}
	return lines
}

type fileKey struct {
	File string
	Key  string
}

func entryKey(entry Entry) fileKey {
	return fileKey{File: entry.File, Key: entry.Key}
}

func anomalyReason(entry Entry) string {
	switch entry.Status {
	case StatusMissingKey:
		return "no such key in this file"
	case StatusAmbiguous:
		return fmt.Sprintf("%d appears more than once", entry.Base)
	case StatusForeignHost:
		return "points at another host, which no job here serves"
	case StatusSecureScheme:
		return "https — the run proxy serves plain HTTP"
	case StatusNotFound, StatusRewrite, StatusUnchanged:
	}
	return fmt.Sprintf("no %d to shift in the value", entry.Base)
}

func filesOf(entries []Entry) []string {
	seen := map[string]bool{}
	var files []string
	for _, entry := range entries {
		if !seen[entry.File] {
			seen[entry.File] = true
			files = append(files, entry.File)
		}
	}
	sort.Strings(files)
	return files
}

type fileRuleParams struct {
	File  string
	Width int
}

func fileRule(params fileRuleParams) string {
	head := fmt.Sprintf(fileRuleFmt, params.File)
	if fill := params.Width - len([]rune(head)); fill > 0 {
		return head + strings.Repeat(ruleRune, fill)
	}
	return head
}

func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-len([]rune(s))))
}

func changedOwned(owned []OwnedEntry) int {
	count := 0
	for _, entry := range owned {
		if entry.Changed {
			count++
		}
	}
	return count
}

type pluralParams struct {
	Count int
	One   string
	Many  string
}

func plural(params pluralParams) string {
	if params.Count == 1 {
		return params.One
	}
	return params.Many
}
