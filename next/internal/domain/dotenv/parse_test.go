package dotenv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachLineIsClassified(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Line
	}{
		{"plain pair", "KEY=value", Line{Kind: Pair, Key: "KEY", Value: "value", Raw: "KEY=value"}},
		{"export pair", "export FOO=bar", Line{Kind: Pair, Key: "FOO", Value: "bar", Export: true, Raw: "export FOO=bar"}},
		{"hash inside quotes is not a comment", `KEY="a # b"`, Line{Kind: Pair, Key: "KEY", Value: "a # b", Raw: `KEY="a # b"`}},
		{"inline comment stripped from value", "KEY=val # comment", Line{Kind: Pair, Key: "KEY", Value: "val", Raw: "KEY=val # comment"}},
		{"only first equals splits key", "URL=postgres://a=b", Line{Kind: Pair, Key: "URL", Value: "postgres://a=b", Raw: "URL=postgres://a=b"}},
		{"single quoted value", "KEY='raw value'", Line{Kind: Pair, Key: "KEY", Value: "raw value", Raw: "KEY='raw value'"}},
		{"a carriage return is not part of the value", "KEY=value\r", Line{Kind: Pair, Key: "KEY", Value: "value", Raw: "KEY=value\r"}},
		{"full line comment", "# a comment", Line{Kind: Comment, Raw: "# a comment"}},
		{"blank line", "", Line{Kind: Blank, Raw: ""}},
		{"degenerate line is a comment", "just some prose", Line{Kind: Comment, Raw: "just some prose"}},
		{"leading digit key is degenerate", "1KEY=value", Line{Kind: Comment, Raw: "1KEY=value"}},
		{"an unterminated quote is kept as a comment", `KEY="open`, Line{Kind: Comment, Raw: `KEY="open`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := Parse([]byte(tc.content))

			require.Len(t, lines, 1)
			assert.Equal(t, tc.want, lines[0])
		})
	}
}

func TestAByteOrderMarkDoesNotHideTheFirstKey(t *testing.T) {
	lines := Parse([]byte("\ufeffBOM=1\n"))

	assert.Equal(t, Line{Kind: Pair, Key: "BOM", Value: "1", Raw: "\ufeffBOM=1"}, lines[0])
}

func TestAQuotedValueSpansLines(t *testing.T) {
	lines := Parse([]byte("CERT=\"-----BEGIN-----\nMIIB\n-----END-----\"\nNEXT=1"))

	require.Len(t, lines, 2)
	assert.Equal(t, "-----BEGIN-----\nMIIB\n-----END-----", lines[0].Value)
	assert.Equal(t, "NEXT", lines[1].Key)
}
