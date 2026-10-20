package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
)

// entities escape what a quoted label cannot hold as itself.
var entities = strings.NewReplacer(`"`, "#quot;", "<", "#lt;", ">", "#gt;")

// drawMermaid draws the forest read as a top-down Mermaid flowchart: a node per branch, its id its place in the read so that no two branches share one, its label the branch and its badges, an edge from each parent to its child, then the warnings as comments.
func drawMermaid(w io.Writer, read tree.Outcome) {
	ids := map[string]string{}
	for i, item := range read.Items {
		ids[item.Subject] = "n" + strconv.Itoa(i)
	}

	fmt.Fprintln(w, "flowchart TD")
	for _, item := range read.Items {
		label := []string{item.Subject}
		for _, b := range badgesOf(item) {
			label = append(label, b.plain())
		}
		fmt.Fprintf(w, "  %s[\"%s\"]\n", ids[item.Subject], entities.Replace(strings.Join(label, " ")))
		if item.Detail.Parent != nil {
			fmt.Fprintf(w, "  %s --> %s\n", ids[*item.Detail.Parent], ids[item.Subject])
		}
	}
	for _, warning := range read.Warnings {
		fmt.Fprintln(w, "  %% ! "+strings.ReplaceAll(warning.Message, "\n", " "))
	}
}
