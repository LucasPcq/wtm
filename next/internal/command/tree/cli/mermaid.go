package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// entities escape what a quoted label cannot hold as itself, # first being how an entity starts.
var entities = strings.NewReplacer("#", "#35;", `"`, "#quot;", "<", "#lt;", ">", "#gt;", "`", "#96;")

// drawMermaid draws the forest read as a top-down Mermaid flowchart: a node per branch, its id its place in the read so that no two branches share one, its label the branch and its badges, an edge from each parent to its child, then as comments why a worktree was not read and the warnings.
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
	for _, item := range read.Items {
		switch {
		case item.Err != nil:
			fmt.Fprintln(w, "  %% "+output.GlyphFailure+" "+oneLine(item.Err.Error()))
		case item.Status == domain.StatusCancelled:
			fmt.Fprintln(w, "  %% "+output.GlyphFailure+" "+item.Subject+" interrupted")
		}
	}
	for _, warning := range read.Warnings {
		fmt.Fprintln(w, "  %% "+output.GlyphAttention+" "+oneLine(warning.Message))
	}
}

func oneLine(text string) string {
	return strings.ReplaceAll(text, "\n", " ")
}
