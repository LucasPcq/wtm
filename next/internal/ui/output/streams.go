package output

import (
	"bytes"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const dumbTerminal = "dumb"

type Streams struct {
	In         io.Reader
	Out        *Stream
	Err        *Stream
	inTerminal bool
	dumb       bool
}

type Stream struct {
	io.Writer
	terminal bool
	width    func() int
	styles   *Styles
}

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

func Test() (*Streams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	return &Streams{In: in, Out: bufferStream(out), Err: bufferStream(errOut)}, in, out, errOut
}

func bufferStream(buf *bytes.Buffer) *Stream {
	return &Stream{Writer: buf, width: func() int { return 0 }, styles: newStyles(lipgloss.NewRenderer(buf))}
}

func (s *Streams) SetStdinTerminal(on bool) { s.inTerminal = on }

func (s *Streams) SetDumbTerminal(on bool) { s.dumb = on }

func (s *Streams) CanPrompt() bool {
	return s.inTerminal && s.CanAnimate()
}

func (s *Streams) CanAnimate() bool {
	return s.Err.terminal && !s.dumb
}

func (s *Stream) IsTerminal() bool { return s.terminal }

func (s *Stream) SetTerminal(on bool) { s.terminal = on }

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

func StylesOf(w io.Writer) *Styles {
	if stream, _ := streamOf(w); stream != nil {
		return stream.styles
	}
	return plainStyles
}

func IsTerminal(w io.Writer) bool {
	stream, _ := streamOf(w)
	return stream != nil && stream.terminal
}

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
