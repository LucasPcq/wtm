package output

import (
	"bytes"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const dumbTerminal = "dumb"

// Streams are the standard streams a command reads and writes, with what it may do on them: prompt, animate, colour.
// A command never writes to os.Stdout or os.Stderr itself, so a test substitutes them with [Test].
type Streams struct {
	In         io.Reader
	Out        *Stream
	Err        *Stream
	inTerminal bool
	dumb       bool
}

// A Stream is an output stream that knows whether it is a terminal, its width, and the colour renderer its styles draw with.
type Stream struct {
	io.Writer
	terminal bool
	width    func() int
	styles   *Styles
}

// System returns the process's own streams, each a terminal or not as the operating system says, $TERM=dumb included.
func System() *Streams {
	return &Streams{
		In:         os.Stdin,
		Out:        fileStream(os.Stdout),
		Err:        fileStream(os.Stderr),
		inTerminal: term.IsTerminal(int(os.Stdin.Fd())),
		dumb:       os.Getenv("TERM") == dumbTerminal,
	}
}

func fileStream(file *os.File) *Stream {
	fd := int(file.Fd())
	width := func() int {
		cols, _, err := term.GetSize(fd)
		if err != nil {
			return 0
		}
		return cols
	}

	return &Stream{Writer: file, terminal: term.IsTerminal(fd), width: width, styles: newStyles(lipgloss.NewRenderer(file))}
}

// Test returns streams over buffers, none a terminal, and the buffers behind stdin, stdout and stderr.
func Test() (*Streams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	return &Streams{In: in, Out: bufferStream(out), Err: bufferStream(errOut)}, in, out, errOut
}

func bufferStream(buf *bytes.Buffer) *Stream {
	return &Stream{Writer: buf, width: func() int { return 0 }, styles: newStyles(lipgloss.NewRenderer(buf))}
}

// SetStdinTerminal makes stdin count as a terminal or not, for a test.
func (s *Streams) SetStdinTerminal(on bool) { s.inTerminal = on }

// SetDumbTerminal makes the terminal count as $TERM=dumb or not, for a test.
func (s *Streams) SetDumbTerminal(on bool) { s.dumb = on }

// CanPrompt reports whether a form may ask questions: stdin is a terminal and [Streams.CanAnimate] holds, since the form draws on stderr.
func (s *Streams) CanPrompt() bool {
	return s.inTerminal && s.CanAnimate()
}

// CanAnimate reports whether progress may redraw itself on stderr: stderr is a terminal that is not $TERM=dumb.
func (s *Streams) CanAnimate() bool {
	return s.Err.terminal && !s.dumb
}

// IsTerminal reports whether s is a terminal.
func (s *Stream) IsTerminal() bool { return s.terminal }

// SetTerminal makes s count as a terminal or not, for a test; its width stays unknown.
func (s *Stream) SetTerminal(on bool) { s.terminal = on }

// streamOf returns the [Stream] under w, through the frames wrapped around it, and how many frames' bars stand before it; nil when w writes to no Stream.
func streamOf(w io.Writer) (*Stream, int) {
	bars := 0
	for {
		switch v := w.(type) {
		case *Stream:
			return v, bars
		case *barWriter:
			w, bars = v.w, bars+1
		default:
			return nil, bars
		}
	}
}

// StylesOf returns the styles of the stream under w, or styles that draw no colour when w writes to no [Stream].
func StylesOf(w io.Writer) *Styles {
	if stream, _ := streamOf(w); stream != nil {
		return stream.styles
	}
	return plainStyles
}

// IsTerminal reports whether w writes to a [Stream] that is a terminal, through any frame.
func IsTerminal(w io.Writer) bool {
	stream, _ := streamOf(w)
	return stream != nil && stream.terminal
}

// terminalWidth returns the columns left to w on its terminal once the bars of its frames are drawn, or 0 when the width is unknown.
func terminalWidth(w io.Writer) int {
	stream, bars := streamOf(w)
	if stream == nil || !stream.terminal {
		return 0
	}

	cols := stream.width()
	if cols <= 0 {
		return 0
	}

	return max(cols-bars*accentBarWidth, 0)
}

// Print draws what draw writes onto s in one write, with s's styles, and returns that write's error: a line of a stream whose reader may leave.
func (s *Stream) Print(draw func(io.Writer)) error {
	var buf bytes.Buffer
	draw(&Stream{Writer: &buf, terminal: s.terminal, width: s.width, styles: s.styles})
	_, err := s.Write(buf.Bytes())
	return err
}
