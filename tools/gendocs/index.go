package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// indexFile lists the command reference by the groups of `wtm --help`, so the
// documentation site can lay out its sidebar the way the CLI presents itself.
const (
	indexFile          = "commands.json"
	ungroupedGroupID   = "other"
	ungroupedGroupName = "Other"
)

type commandIndex struct {
	Groups []commandGroup `json:"groups"`
}

type commandGroup struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Commands []commandEntry `json:"commands"`
}

type commandEntry struct {
	Path     string         `json:"path"`
	File     string         `json:"file"`
	Short    string         `json:"short"`
	Commands []commandEntry `json:"commands,omitempty"`
}

func writeIndex(root *cobra.Command) error {
	data, err := json.MarshalIndent(buildIndex(root), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, indexFile), append(data, '\n'), 0o644)
}

func buildIndex(root *cobra.Command) commandIndex {
	groups := make([]commandGroup, 0, len(root.Groups())+1)
	for _, g := range root.Groups() {
		groups = append(groups, commandGroup{ID: g.ID, Title: strings.TrimSuffix(g.Title, ":")})
	}
	groups = append(groups, commandGroup{ID: ungroupedGroupID, Title: ungroupedGroupName})

	for _, c := range documented(root) {
		i := groupIndex(groups, c.GroupID)
		groups[i].Commands = append(groups[i].Commands, entryFor(c))
	}

	nonEmpty := groups[:0]
	for _, g := range groups {
		if len(g.Commands) > 0 {
			nonEmpty = append(nonEmpty, g)
		}
	}
	return commandIndex{Groups: nonEmpty}
}

func groupIndex(groups []commandGroup, id string) int {
	for i, g := range groups {
		if g.ID == id {
			return i
		}
	}
	return len(groups) - 1
}

func entryFor(c *cobra.Command) commandEntry {
	children := documented(c)
	entries := make([]commandEntry, 0, len(children))
	for _, child := range children {
		entries = append(entries, entryFor(child))
	}
	return commandEntry{
		Path:     c.CommandPath(),
		File:     strings.ReplaceAll(c.CommandPath(), " ", "_") + ".md",
		Short:    c.Short,
		Commands: entries,
	}
}

// documented mirrors the filter of cobra/doc.GenMarkdownTree, so the index never
// names a page the generator did not write.
func documented(c *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	for _, child := range c.Commands() {
		if child.IsAvailableCommand() && !child.IsAdditionalHelpTopicCommand() {
			out = append(out, child)
		}
	}
	return out
}
