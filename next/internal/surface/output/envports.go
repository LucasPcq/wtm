// Package output is what wtm-next prints: human frames and blocks, and the JSON envelope.
package output

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/settle"
)

func SettlementNote(plan settle.Redacted) string {
	var parts []string
	if shifted := len(settle.Plan(plan).Rewrites()); shifted > 0 {
		parts = append(parts, fmt.Sprintf("%d %s settled (offset +%d)", shifted, plural(pluralParams{Count: shifted, One: "port", Many: "ports"}), plan.Offset))
	}
	if written := changedOwned(plan.Owned); written > 0 {
		parts = append(parts, fmt.Sprintf("%d owned %s written", written, plural(pluralParams{Count: written, One: "value", Many: "values"})))
	}
	return strings.Join(parts, " · ")
}

func AnomalyLines(plan settle.Redacted) []string {
	entries := settle.Plan(plan).Anomalies()
	if len(entries) == 0 {
		return nil
	}
	keyWidth, portWidth := 0, 0
	for _, entry := range entries {
		keyWidth = max(keyWidth, len(entry.Key))
		portWidth = max(portWidth, len(entry.Port))
	}
	rows := make([]string, len(entries))
	widest := 0
	for i, entry := range entries {
		rows[i] = strings.TrimRight(fmt.Sprintf("%s  %s  %s", pad(entry.Key, keyWidth), pad(entry.Port, portWidth), anomalyReason(entry)), " ")
		widest = max(widest, len([]rune(rows[i])))
	}
	var lines []string
	for i, file := range filesOf(entries) {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fileRule(fileRuleParams{File: file, Width: widest}))
		for j, entry := range entries {
			if entry.File == file {
				lines = append(lines, rows[j])
			}
		}
	}
	return lines
}

func anomalyReason(entry settle.Entry) string {
	switch entry.Status {
	case settle.StatusMissingKey:
		return "no such key in this file"
	case settle.StatusAmbiguous:
		return fmt.Sprintf("%d appears more than once", entry.Base)
	case settle.StatusForeignHost:
		return "points at another host, which no job here serves"
	case settle.StatusSecureScheme:
		return "https — the run proxy serves plain HTTP"
	case settle.StatusNotFound, settle.StatusRewrite, settle.StatusUnchanged:
	}
	return fmt.Sprintf("no %d to shift in the value", entry.Base)
}

func filesOf(entries []settle.Entry) []string {
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
	head := fmt.Sprintf("── %s ", params.File)
	if fill := params.Width - len([]rune(head)); fill > 0 {
		return head + strings.Repeat("─", fill)
	}
	return head
}

func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-len([]rune(s))))
}

func changedOwned(owned []settle.OwnedEntry) int {
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
