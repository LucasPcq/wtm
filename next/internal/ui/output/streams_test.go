package output

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARunPromptsOnlyWhenStdinAndStderrAreTerminalsOfAKnownKind(t *testing.T) {
	cases := map[string]struct {
		In, Out, Err, Dumb bool
		Prompt, Animate    bool
	}{
		"every stream a terminal":   {In: true, Out: true, Err: true, Prompt: true, Animate: true},
		"stdout piped":              {In: true, Err: true, Prompt: true, Animate: true},
		"stdin piped":               {Out: true, Err: true, Animate: true},
		"stderr redirected":         {In: true, Out: true},
		"a terminal with TERM=dumb": {In: true, Out: true, Err: true, Dumb: true},
		"no terminal":               {},
	}
	for name, c := range cases {
		streams, _, _, _ := Test()
		streams.SetStdinTerminal(c.In)
		streams.Out.SetTerminal(c.Out)
		streams.Err.SetTerminal(c.Err)
		streams.SetDumbTerminal(c.Dumb)

		assert.Equal(t, c.Prompt, streams.CanPrompt(), name)
		assert.Equal(t, c.Animate, streams.CanAnimate(), name)
	}
}

func TestAFrameBarsItsLinesOnATerminalOnly(t *testing.T) {
	for _, terminal := range []bool{true, false} {
		streams, _, out, _ := Test()
		streams.Out.SetTerminal(terminal)

		Frame(streams.Out, func(w io.Writer) { Message(w, "body") })

		want := "\n  body\n\n"
		if terminal {
			want = "\n┃  body\n\n"
		}
		assert.Equal(t, want, out.String())
	}
}

func TestAStyleFollowsTheStreamItIsDrawnOn(t *testing.T) {
	colour := lipgloss.NewRenderer(io.Discard)
	colour.SetColorProfile(termenv.ANSI)
	var coloured, plain bytes.Buffer
	onTerminal := &Stream{Writer: &coloured, terminal: true, width: func() int { return 0 }, styles: newStyles(colour)}
	piped := bufferStream(&plain)

	Frame(onTerminal, func(w io.Writer) { ErrorLine(w, "failed") })
	Frame(piped, func(w io.Writer) { ErrorLine(w, "failed") })

	assert.Contains(t, coloured.String(), "\x1b[")
	assert.Equal(t, "\n  ✗ failed\n\n", plain.String())
}

func TestAStreamOfTheProcessThatIsNoTerminalDrawsNoColour(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	t.Setenv("CLICOLOR_FORCE", "")
	stream := fileStream(w)

	assert.False(t, stream.IsTerminal())
	assert.Equal(t, termenv.Ascii, stream.styles.renderer.ColorProfile())
	assert.Zero(t, terminalWidth(stream))
}

func TestAWriterThatIsNoStreamIsPlain(t *testing.T) {
	var out bytes.Buffer

	assert.False(t, IsTerminal(&out))
	assert.Same(t, plainStyles, StylesOf(&out))
}

func TestAPrintIsOneWriteThatReportsItsError(t *testing.T) {
	broken := &Stream{Writer: failingWriter{}, width: func() int { return 0 }, styles: plainStyles}
	streams, _, out, _ := Test()

	require.NoError(t, streams.Out.Print(func(w io.Writer) { SuccessLine(w, "created") }))
	err := broken.Print(func(w io.Writer) { SuccessLine(w, "created") })

	assert.Equal(t, "  ✓ created\n", out.String())
	assert.EqualError(t, err, "reader gone")
}
