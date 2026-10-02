package output

import (
	"io"

	"github.com/LucasPcq/wtm/internal/styles"
)

func Message(w io.Writer, line string) { _, _ = io.WriteString(w, line) }

func render(w io.Writer) {
	Message(w, styles.Muted.Render("nothing to do")) // want `a bare line muted whole is the .=. register without its glyph`
	Message(w, styles.Muted.Render("label")+" value")
	Message(w, styles.Accent.Render("x"))
}
