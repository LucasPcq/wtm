package dotenv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAFileRoundTripsByteForByte(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"simple pair with newline", "KEY=value\n"},
		{"no trailing newline", "KEY=value"},
		{"export prefix", "export KEY=value\n"},
		{"leading indentation", "  KEY=value\n"},
		{"double quoted", "KEY=\"hello world\"\n"},
		{"single quoted", "KEY='hello world'\n"},
		{"value containing equals", "URL=postgres://user:pw@host/db?a=b\n"},
		{"hash inside quotes", "KEY=\"a # b\"\n"},
		{"full and inline comments", "# header\nKEY=value # trailing\n"},
		{"blank line between pairs", "A=1\n\nB=2\n"},
		{"multiple trailing blanks", "A=1\n\n\n"},
		{"multiline quoted value", "CERT=\"-----BEGIN-----\nMIIBcontent\n-----END-----\"\n"},
		{"multiline json value", "CONFIG='{\n  \"a\": 1\n}'\n"},
		{"degenerate no equals", "just some prose\nKEY=1\n"},
		{"crlf line endings", "A=1\r\nB=2\r\n"},
		{"empty content", ""},
		{"only blank lines", "\n\n"},
		{"empty value", "KEY=\n"},
		{"internal whitespace unquoted", "KEY=hello world\n"},
		{"escaped quote in double quotes", "KEY=\"he said \\\"hi\\\"\"\n"},
		{"everything at once, CRLF, no final newline", "# app settings\r\n\r\nexport API_PORT=4002 # api\r\nDATABASE_URL=\"postgres://u:p@localhost:5432/db?sslmode=disable\"\r\nREDIS_URL='redis://u:pa$w0rd@localhost:6379/0'\r\n\r\nGREETING=hello world\r\nEMPTY=\r\n  INDENTED = spaced\r\nnot a pair\r\nLAST=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.content, string(Render(Parse([]byte(tc.content)))))
		})
	}
}

func TestANewPairIsRenderedCanonically(t *testing.T) {
	cases := []struct {
		name string
		line Line
		want string
	}{
		{"bare value needs no quotes", Line{Kind: Pair, Key: "KEY", Value: "simple"}, "KEY=simple"},
		{"value with space is quoted", Line{Kind: Pair, Key: "KEY", Value: "hello world"}, `KEY="hello world"`},
		{"export prefix preserved", Line{Kind: Pair, Key: "FOO", Value: "bar", Export: true}, "export FOO=bar"},
		{"value with equals stays bare", Line{Kind: Pair, Key: "KEY", Value: "a=b"}, "KEY=a=b"},
		{"inline-comment-like value is quoted", Line{Kind: Pair, Key: "KEY", Value: "val # x"}, `KEY="val # x"`},
		{"embedded double quote is escaped", Line{Kind: Pair, Key: "KEY", Value: `say "hi"`}, `KEY="say \"hi\""`},
		{"a dollar takes single quotes", Line{Kind: Pair, Key: "PW", Value: "pa$w0rd"}, "PW='pa$w0rd'"},
		{"empty value stays bare", Line{Kind: Pair, Key: "KEY", Value: ""}, "KEY="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, string(Render([]Line{tc.line})))
		})
	}
}

func TestANewPairLeavesItsNeighboursAsWritten(t *testing.T) {
	lines := []Line{
		{Kind: Comment, Raw: "# keep me"},
		{Kind: Pair, Key: "A", Value: "1", Raw: "A=1   # aligned"},
		{Kind: Pair, Key: "B", Value: "new value"},
		{Kind: Blank, Raw: ""},
	}

	assert.Equal(t, "# keep me\nA=1   # aligned\nB=\"new value\"\n", string(Render(lines)))
}
