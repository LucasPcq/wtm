package output

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAFrameIsOneBlockBetweenTwoBlankLines(t *testing.T) {
	assert.Equal(t, "\n  body\n\n", rendered(func(w *bytes.Buffer) { Frame(w, func(body io.Writer) { message(body, "body") }) }))
}

func TestTheBarMarksEveryLineOnceCRLFIncludedAcrossWrites(t *testing.T) {
	var out bytes.Buffer
	bar := &barWriter{w: &out, atLineStart: true}

	_, _ = bar.Write([]byte("one\r"))
	_, _ = bar.Write([]byte("\ntwo\n"))

	assert.Equal(t, "┃one\r\n┃two\n", out.String())
}
