package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const columnGap = "  "

// Columns writes rows to w as an aligned table, one line per row: each cell padded to the widest of its column as the terminal draws it, escape codes and wide runes counted for the columns they take, and nothing after a line's last cell.
func Columns(w io.Writer, rows [][]string) {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], ansi.StringWidth(cell))
		}
	}

	for _, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i > 0 {
				line.WriteString(columnGap)
			}
			line.WriteString(cell + strings.Repeat(" ", widths[i]-ansi.StringWidth(cell)))
		}
		fmt.Fprintf(w, "%s%s\n", Indent, strings.TrimRight(line.String(), " "))
	}
}
