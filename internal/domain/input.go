package domain

// KeyStroke is one keypress in terms a rule can encode without knowing which
// toolkit read it: a named key ("enter", "ctrl+c", "up"), or the characters a
// text key produced. Alt is the modifier a terminal sends as a leading escape.
type KeyStroke struct {
	Name  string
	Runes []rune
	Alt   bool
}
