package output

import (
	"fmt"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

func settlementNote(plan env.Redacted) string {
	var parts []string
	if shifted := len(env.Plan(plan).Rewrites()); shifted > 0 {
		parts = append(parts, fmt.Sprintf("%d %s settled (offset +%d)", shifted, noun(shifted, "port"), plan.Offset))
	}
	if written := changedOwned(plan.Owned); written > 0 {
		parts = append(parts, fmt.Sprintf("%d owned %s written", written, noun(written, "value")))
	}
	return strings.Join(parts, dot)
}

func anomalyLines(plan env.Redacted) []string {
	entries := env.Plan(plan).Anomalies()
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
		head := fmt.Sprintf("── %s ", file)
		lines = append(lines, head+strings.Repeat("─", max(widest-len([]rune(head)), 0)))
		for j, entry := range entries {
			if entry.File == file {
				lines = append(lines, rows[j])
			}
		}
	}
	return lines
}

func anomalyReason(entry env.Entry) string {
	switch entry.Status {
	case env.StatusMissingKey:
		return "no such key in this file"
	case env.StatusAmbiguous:
		return fmt.Sprintf("%d appears more than once", entry.Base)
	case env.StatusForeignHost:
		return "points at another host, which no job here serves"
	case env.StatusSecureScheme:
		return "https — the run proxy serves plain HTTP"
	case env.StatusNotFound, env.StatusRewrite, env.StatusUnchanged:
	}
	return fmt.Sprintf("no %d to shift in the value", entry.Base)
}

func filesOf(entries []env.Entry) []string {
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, entry.File)
	}
	slices.Sort(files)
	return slices.Compact(files)
}

func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-len([]rune(s))))
}

func changedOwned(owned []env.OwnedEntry) int {
	count := 0
	for _, entry := range owned {
		if entry.Changed {
			count++
		}
	}
	return count
}

func noun(count int, one string) string {
	if count == 1 {
		return one
	}
	return one + "s"
}
