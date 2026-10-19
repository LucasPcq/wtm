package provision

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

// portsLeftAlone warns of the ports plan could not shift, a table per .env file under the title "Env ports left alone"; none when there are none.
func portsLeftAlone(plan env.Redacted) []domain.Warning {
	lines := anomalyLines(plan)
	if len(lines) == 0 {
		return nil
	}

	return []domain.Warning{{
		Code:    "provision.env_ports_left_alone",
		Message: "Env ports left alone\n" + strings.Join(lines, "\n"),
		Params:  domain.Params{"count": strconv.Itoa(len(env.Plan(plan).Anomalies()))},
	}}
}

// anomalyLines returns the ports plan could not shift, as a table per .env file: each key, its port and why; nil when there are none.
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
