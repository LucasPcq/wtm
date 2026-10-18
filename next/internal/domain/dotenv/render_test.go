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
		{"simple_pair_newline", "KEY=value\n"},
		{"no_trailing_newline", "KEY=value"},
		{"export_prefix", "export KEY=value\n"},
		{"leading_indentation", "  KEY=value\n"},
		{"double_quoted", "KEY=\"hello world\"\n"},
		{"single_quoted", "KEY='hello world'\n"},
		{"value_with_equals", "URL=postgres://user:pw@host/db?a=b\n"},
		{"hash_inside_quotes", "KEY=\"a # b\"\n"},
		{"full_and_inline_comments", "# header\nKEY=value # trailing\n"},
		{"blank_line_between_pairs", "A=1\n\nB=2\n"},
		{"multiple_trailing_blanks", "A=1\n\n\n"},
		{"multiline_quoted", "CERT=\"-----BEGIN-----\nMIIBcontent\n-----END-----\"\n"},
		{"multiline_json", "CONFIG='{\n  \"a\": 1\n}'\n"},
		{"degenerate_no_equals", "just some prose\nKEY=1\n"},
		{"crlf_line_endings", "A=1\r\nB=2\r\n"},
		{"empty_content", ""},
		{"only_blank_lines", "\n\n"},
		{"empty_value", "KEY=\n"},
		{"internal_whitespace_unquoted", "KEY=hello world\n"},
		{"escaped_quote_in_double", "KEY=\"he said \\\"hi\\\"\"\n"},
		{"mostly_crlf_one_lf", "A=1\r\nB=2\r\nC=3\n"},
		{"lf_before_crlf_multiline", "U=x\nM=\"multi\r\nline\"\r\n"},
		{"byte_order_mark", "\ufeffBOM=1\nA=2\n"},
		{"everything_crlf_no_final_newline", "# app settings\r\n\r\nexport API_PORT=4002 # api\r\nDATABASE_URL=\"postgres://u:p@localhost:5432/db?sslmode=disable\"\r\nREDIS_URL='redis://u:pa$w0rd@localhost:6379/0'\r\n\r\nGREETING=hello world\r\nEMPTY=\r\n  INDENTED = spaced\r\nnot a pair\r\nLAST=1"},
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
		{"bare_needs_no_quotes", Line{Kind: Pair, Key: "KEY", Value: "simple"}, "KEY=simple"},
		{"space_quoted", Line{Kind: Pair, Key: "KEY", Value: "hello world"}, `KEY="hello world"`},
		{"export_prefix_preserved", Line{Kind: Pair, Key: "FOO", Value: "bar", Export: true}, "export FOO=bar"},
		{"equals_stays_bare", Line{Kind: Pair, Key: "KEY", Value: "a=b"}, "KEY=a=b"},
		{"comment_like_quoted", Line{Kind: Pair, Key: "KEY", Value: "val # x"}, `KEY="val # x"`},
		{"double_quote_takes_single", Line{Kind: Pair, Key: "KEY", Value: `say "hi"`}, `KEY='say "hi"'`},
		{"dollar_takes_single", Line{Kind: Pair, Key: "PW", Value: "pa$w0rd"}, "PW='pa$w0rd'"},
		{"empty_stays_bare", Line{Kind: Pair, Key: "KEY", Value: ""}, "KEY="},
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
