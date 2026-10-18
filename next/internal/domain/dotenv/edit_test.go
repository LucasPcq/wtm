package dotenv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestANewValueKeepsTheLineAsWritten(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		value string
		want  string
	}{
		{"bare", "PORT=4002", "4012", "PORT=4012"},
		{"single_quoted_dollar_stays", "REDIS_URL='redis://u:pa$w0rd@localhost:6379/0'", "redis://u:pa$w0rd@localhost:6389/0", "REDIS_URL='redis://u:pa$w0rd@localhost:6389/0'"},
		{"double_quoted_inline_comment", `MINIO_URL="http://localhost:9000" # minio endpoint`, "http://localhost:9010", `MINIO_URL="http://localhost:9010" # minio endpoint`},
		{"bare_inline_comment", "ADMINER_PORT=8080 # adminer ui", "8090", "ADMINER_PORT=8090 # adminer ui"},
		{"export_prefix_indentation", "  export API_PORT=4002", "4012", "  export API_PORT=4012"},
		{"spaced_assignment", "API_PORT = 4002", "4012", "API_PORT = 4012"},
		{"carriage_return_stays", "PORT=4002\r", "4012", "PORT=4012\r"},
		{"quoted_carriage_return", "URL='http://localhost:4002'\r", "http://localhost:4012", "URL='http://localhost:4012'\r"},
		{"single_quoted_to_hostname", "VITE_API_URL='http://localhost:4002/api'", "http://api.feat-x.monorepo.localhost:11080/api", "VITE_API_URL='http://api.feat-x.monorepo.localhost:11080/api'"},
		{"empty_value_replaced", "COMPOSE_PROJECT_NAME=", "monorepo-feat-x", "COMPOSE_PROJECT_NAME=monorepo-feat-x"},
		{"empty_value_before_comment", "COMPOSE_PROJECT_NAME= # set by wtm", "monorepo-feat-x", "COMPOSE_PROJECT_NAME=monorepo-feat-x # set by wtm"},
		{"bare_needing_quotes_quoted", "GREETING=hello", "hello world", `GREETING="hello world"`},
		{"bare_gaining_dollar_single_quoted", "PW=secret", "pa$w0rd", "PW='pa$w0rd'"},
		{"single_quoted_gaining_quote_falls_back", "Q='a'", "it's", "Q=it's"},
		{"bare_spaced_stays_bare", "DSN=host=localhost port=3000 dbname=app", "host=localhost port=3010 dbname=app", "DSN=host=localhost port=3010 dbname=app"},
		{"bare_spaced_keeps_comment", "DSN=host=db port=3000 # libpq", "host=db port=3010", "DSN=host=db port=3010 # libpq"},
		{"double_quoted_spaced_stays", `DSN="host=localhost port=3000"`, "host=localhost port=3010", `DSN="host=localhost port=3010"`},
		{"single_quoted_spaced_stays", "DSN='host=localhost port=3000'", "host=localhost port=3010", "DSN='host=localhost port=3010'"},
		{"bare_spaced_gaining_hash_quoted", "DSN=a b", "a #b", `DSN="a #b"`},
		{"bare_spaced_gaining_edge_space_quoted", "DSN=a b", "a b ", `DSN="a b "`},
		{"bare_spaced_gaining_newline_quoted", "DSN=a b", "a\nb", "DSN=\"a\nb\""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := onlyPair(t, tc.line).WithValue(tc.value)

			assert.Equal(t, tc.want, got.Raw)
			assert.Equal(t, tc.value, got.Value)
			assert.Equal(t, tc.value, onlyPair(t, got.Raw).Value)
		})
	}
}

func TestTheSameValueLeavesTheLineAlone(t *testing.T) {
	line := onlyPair(t, "A='x'   # c\r")

	assert.Equal(t, line, line.WithValue("x"))
}

func TestSettingAKeyTheFileHoldsRewritesItInPlace(t *testing.T) {
	lines := Parse([]byte("A=1\nCOMPOSE_PROJECT_NAME=repo # wtm\nB=2\n"))

	out, changed := Set(SetParams{Lines: lines, Key: "COMPOSE_PROJECT_NAME", Value: "repo-feat-x"})

	assert.True(t, changed)
	assert.Equal(t, "A=1\nCOMPOSE_PROJECT_NAME=repo-feat-x # wtm\nB=2\n", string(Render(out)))
	assert.Equal(t, "repo", lines[1].Value, "the input is not changed")
}

func TestSettingAKeyTheFileLacksAddsItAfterItsLastContent(t *testing.T) {
	out, changed := Set(SetParams{Lines: Parse([]byte("A=1\n\n")), Key: "B", Value: "2"})

	assert.True(t, changed)
	assert.Equal(t, "A=1\nB=2\n\n", string(Render(out)))
}

func TestSettingTheValueAKeyHoldsChangesNothing(t *testing.T) {
	lines := Parse([]byte("A=1\n"))

	out, changed := Set(SetParams{Lines: lines, Key: "A", Value: "1"})

	assert.False(t, changed)
	assert.Equal(t, lines, out)
}

func TestACRLFFileStaysCRLFThroughEdits(t *testing.T) {
	lines := Parse([]byte("# header\r\nA=1\r\nPORT=4002\r\n\r\nB='b'\r\n"))
	lines, _ = Set(SetParams{Lines: lines, Key: "PORT", Value: "4012"})
	lines, _ = Set(SetParams{Lines: lines, Key: "COMPOSE_PROJECT_NAME", Value: "p"})

	assert.Equal(t, "# header\r\nA=1\r\nPORT=4012\r\n\r\nB='b'\r\nCOMPOSE_PROJECT_NAME=p\r\n", string(Render(lines)))
}

func TestAnLFFileStaysLFThroughEdits(t *testing.T) {
	lines, _ := Set(SetParams{Lines: Parse([]byte("A=1\n")), Key: "B", Value: "2"})

	assert.Equal(t, "A=1\nB=2\n", string(Render(lines)))
}

func TestAValueSetOnAPairWithoutItsTextIsRenderedCanonically(t *testing.T) {
	line := Line{Kind: Pair, Key: "A", Value: "1"}.WithValue("hello world")

	require.Empty(t, line.Raw)
	assert.Equal(t, `A="hello world"`, string(Render([]Line{line})))
}

func TestAValueHoldingQuotesReadsBackAsSet(t *testing.T) {
	cases := []struct{ name, value string }{
		{"spaced_double_quotes", `say "hi"`},
		{"inner_double_quote", `a"b`},
		{"leading_double_quote", `"lead`},
		{"apostrophe", `it's`},
		{"dollar_and_double_quote", `pa$w"rd`},
		{"apostrophe_and_double_quote", `it's"fine`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines, changed := Set(SetParams{Lines: Parse([]byte("A=1\n")), Key: "A", Value: tc.value})
			require.True(t, changed)

			assert.Equal(t, tc.value, valueOf(t, Render(lines), "A"))
			_, again := Set(SetParams{Lines: Parse(Render(lines)), Key: "A", Value: tc.value})
			assert.False(t, again, "a second Set of the same value changes nothing")
		})
	}
}

func TestAnAddedKeyIsNotSwallowedByAnEarlierUnterminatedQuote(t *testing.T) {
	lines, _ := Set(SetParams{Lines: Parse([]byte("N='open\n")), Key: "W", Value: "it's"})

	rendered := Render(lines)

	assert.Equal(t, "it's", valueOf(t, rendered, "W"))
	assert.Contains(t, string(rendered), "N='open")
}

func TestAKeyAfterAByteOrderMarkIsSetInPlace(t *testing.T) {
	lines, _ := Set(SetParams{Lines: Parse([]byte("\ufeffBOM=1\n")), Key: "BOM", Value: "2"})

	assert.Equal(t, "\ufeffBOM=2\n", string(Render(lines)))
}

func TestAKeyAddedAfterAnUnterminatedLastLineOfACRLFFileKeepsItCRLF(t *testing.T) {
	lines, _ := Set(SetParams{Lines: Parse([]byte("A=1\r\nB=2")), Key: "C", Value: "3"})

	assert.Equal(t, "A=1\r\nB=2\r\nC=3", string(Render(lines)))
}

func TestAValueNoQuotingCanHoldIsWrittenOnceAndThenLeftAlone(t *testing.T) {
	first, _ := Set(SetParams{Lines: Parse([]byte("A=1\n")), Key: "A", Value: `it's "fine"`})

	_, again := Set(SetParams{Lines: Parse(Render(first)), Key: "A", Value: `it's "fine"`})

	assert.Equal(t, "A=\"it's \\\"fine\\\"\"\n", string(Render(first)))
	assert.False(t, again)
}

func TestAKeyAddedToAByteOrderMarkAloneComesAfterTheMark(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{"mark_only", "\ufeff", "\ufeffK=V\n"},
		{"mark_blank_lines", "\ufeff\n\n", "\ufeffK=V\n\n\n"},
		{"mark_crlf", "\ufeff\r\n", "\ufeffK=V\r\n\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines, _ := Set(SetParams{Lines: Parse([]byte(tc.content)), Key: "K", Value: "V"})

			assert.Equal(t, tc.want, string(Render(lines)))
			assert.Equal(t, "V", valueOf(t, Render(lines), "K"))
		})
	}
}

func TestAKeyMovedToTheTopOfAMarkedFileTakesTheMark(t *testing.T) {
	lines, _ := Set(SetParams{Lines: Parse([]byte("\ufeffN='open\n")), Key: "W", Value: "it's"})

	assert.Equal(t, "\ufeffW=it's\nN='open\n", string(Render(lines)))
}

func TestAKeySetOnNoLinesIsTheWholeFile(t *testing.T) {
	lines, changed := Set(SetParams{Key: "K", Value: "V"})

	assert.True(t, changed)
	assert.Equal(t, "K=V", string(Render(lines)))
}

func TestAKeyJustAddedCanBeSetAgain(t *testing.T) {
	lines, _ := Set(SetParams{Lines: Parse([]byte("A=1\n")), Key: "K", Value: "x"})

	lines, changed := Set(SetParams{Lines: lines, Key: "K", Value: "y"})

	assert.True(t, changed)
	assert.Equal(t, "A=1\nK=y\n", string(Render(lines)))
}

func TestAnEditWritesOnlyWhatChangesAndCreatesOnlyWhenAsked(t *testing.T) {
	add := func(lines []Line) []Line {
		out, _ := Set(SetParams{Lines: lines, Key: "K", Value: "v"})
		return out
	}

	_, absent := Edit{Edit: add}.Over(nil)
	created, create := Edit{Create: true, Edit: add}.Over(nil)
	_, same := Edit{Edit: add}.Over([]byte("K=v\n"))
	changed, change := Edit{Edit: add}.Over([]byte("A=1\n"))

	assert.False(t, absent)
	assert.True(t, create)
	assert.Equal(t, "K=v", string(created))
	assert.False(t, same)
	assert.True(t, change)
	assert.Equal(t, "A=1\nK=v\n", string(changed))
}
