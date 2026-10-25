package formtest

import "time"

// Enters is a terminal on which the user keeps pressing enter.
type Enters struct{}

// Read reads an enter, a millisecond after the last.
func (Enters) Read(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	p[0] = '\r'
	return 1, nil
}

// Escapes is a terminal on which the user keeps pressing esc.
type Escapes struct{}

// Read reads an esc, a millisecond after the last.
func (Escapes) Read(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	p[0] = 0x1b
	return 1, nil
}
