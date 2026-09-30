package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestEncodeKeyStroke(t *testing.T) {
	cases := []struct {
		label  string
		stroke domain.KeyStroke
		want   string
	}{
		{label: "a letter", stroke: domain.KeyStroke{Runes: []rune("a")}, want: "a"},
		{label: "pasted text", stroke: domain.KeyStroke{Runes: []rune("hello")}, want: "hello"},
		{label: "a wide rune", stroke: domain.KeyStroke{Runes: []rune("é")}, want: "é"},
		{label: "interrupt", stroke: domain.KeyStroke{Name: "ctrl+c"}, want: "\x03"},
		{label: "first control", stroke: domain.KeyStroke{Name: "ctrl+a"}, want: "\x01"},
		{label: "last control", stroke: domain.KeyStroke{Name: "ctrl+z"}, want: "\x1a"},
		{label: "enter", stroke: domain.KeyStroke{Name: "enter"}, want: "\r"},
		{label: "tab", stroke: domain.KeyStroke{Name: "tab"}, want: "\t"},
		{label: "shift+tab", stroke: domain.KeyStroke{Name: "shift+tab"}, want: "\x1b[Z"},
		{label: "backspace", stroke: domain.KeyStroke{Name: "backspace"}, want: "\x7f"},
		{label: "space", stroke: domain.KeyStroke{Name: " "}, want: " "},
		{label: "escape", stroke: domain.KeyStroke{Name: "esc"}, want: "\x1b"},
		{label: "arrow", stroke: domain.KeyStroke{Name: "up"}, want: "\x1b[A"},
		{label: "page", stroke: domain.KeyStroke{Name: "pgdown"}, want: "\x1b[6~"},
		{label: "function key", stroke: domain.KeyStroke{Name: "f5"}, want: "\x1b[15~"},
		{label: "punctuation control", stroke: domain.KeyStroke{Name: "ctrl+]"}, want: "\x1d"},
		{label: "alt letter", stroke: domain.KeyStroke{Runes: []rune("b"), Alt: true}, want: "\x1bb"},
		{label: "alt arrow", stroke: domain.KeyStroke{Name: "left", Alt: true}, want: "\x1b\x1b[D"},
		{label: "word-wise motion", stroke: domain.KeyStroke{Name: "ctrl+left"}, want: "\x1b[1;5D"},
		{label: "selection", stroke: domain.KeyStroke{Name: "shift+right"}, want: "\x1b[1;2C"},
		{label: "both modifiers", stroke: domain.KeyStroke{Name: "ctrl+shift+end"}, want: "\x1b[1;6F"},
		{label: "modified page", stroke: domain.KeyStroke{Name: "ctrl+pgup"}, want: "\x1b[5;5~"},
		{label: "high function key", stroke: domain.KeyStroke{Name: "f17"}, want: "\x1b[15;2~"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := string(EncodeKeyStroke(tc.stroke)); got != tc.want {
				t.Fatalf("EncodeKeyStroke(%+v) = %q, want %q", tc.stroke, got, tc.want)
			}
		})
	}
}

// A key with no encoding sends nothing: a stray byte in a job's stdin is worse
// than a key that did not reach it.
func TestEncodeKeyStrokeRefusesWhatItCannotEncode(t *testing.T) {
	unknown := []domain.KeyStroke{
		{},
		{Name: "f21"},
		{Name: "runes"},
		{Name: "ctrl+alt+home"},
	}
	for _, stroke := range unknown {
		if got := EncodeKeyStroke(stroke); got != nil {
			t.Fatalf("EncodeKeyStroke(%+v) = %q, want nothing", stroke, got)
		}
	}
}

// ctrl+home shares the prefix of the control keys without sharing their
// encoding: read as one it would send \x08, which is backspace.
func TestControlByteReadsOnlyLetters(t *testing.T) {
	if _, ok := controlByte("ctrl+home"); ok {
		t.Fatal("ctrl+home was read as a control character")
	}
	if got, ok := controlByte("ctrl+h"); !ok || got != 0x08 {
		t.Fatalf("controlByte(ctrl+h) = %#x, %v", got, ok)
	}
}
