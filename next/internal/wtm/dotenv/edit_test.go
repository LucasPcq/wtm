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
		{"single quotes holding a dollar stay single", "REDIS_URL='redis://u:pa$w0rd@localhost:6379/0'", "redis://u:pa$w0rd@localhost:6389/0", "REDIS_URL='redis://u:pa$w0rd@localhost:6389/0'"},
		{"double quotes and an inline comment", `MINIO_URL="http://localhost:9000" # minio endpoint`, "http://localhost:9010", `MINIO_URL="http://localhost:9010" # minio endpoint`},
		{"bare with an inline comment", "ADMINER_PORT=8080 # adminer ui", "8090", "ADMINER_PORT=8090 # adminer ui"},
		{"export prefix and indentation", "  export API_PORT=4002", "4012", "  export API_PORT=4012"},
		{"spacing around the assignment", "API_PORT = 4002", "4012", "API_PORT = 4012"},
		{"a carriage return stays", "PORT=4002\r", "4012", "PORT=4012\r"},
		{"a quoted value and a carriage return", "URL='http://localhost:4002'\r", "http://localhost:4012", "URL='http://localhost:4012'\r"},
		{"single quotes rewritten to a hostname", "VITE_API_URL='http://localhost:4002/api'", "http://api.feat-x.monorepo.localhost:11080/api", "VITE_API_URL='http://api.feat-x.monorepo.localhost:11080/api'"},
		{"an empty value takes the new one", "COMPOSE_PROJECT_NAME=", "monorepo-feat-x", "COMPOSE_PROJECT_NAME=monorepo-feat-x"},
		{"an empty value before a comment", "COMPOSE_PROJECT_NAME= # set by wtm", "monorepo-feat-x", "COMPOSE_PROJECT_NAME=monorepo-feat-x # set by wtm"},
		{"a bare value needing quotes is quoted", "GREETING=hello", "hello world", `GREETING="hello world"`},
		{"a bare value gaining a dollar is single-quoted", "PW=secret", "pa$w0rd", "PW='pa$w0rd'"},
		{"a single-quoted value gaining a quote falls back", "Q='a'", "it's", "Q=it's"},
		{"a bare value with spaces stays bare", "DSN=host=localhost port=3000 dbname=app", "host=localhost port=3010 dbname=app", "DSN=host=localhost port=3010 dbname=app"},
		{"a bare value with spaces keeps its comment", "DSN=host=db port=3000 # libpq", "host=db port=3010", "DSN=host=db port=3010 # libpq"},
		{"a double-quoted value with spaces stays double", `DSN="host=localhost port=3000"`, "host=localhost port=3010", `DSN="host=localhost port=3010"`},
		{"a single-quoted value with spaces stays single", "DSN='host=localhost port=3000'", "host=localhost port=3010", "DSN='host=localhost port=3010'"},
		{"a bare spaced value gaining a comment mark is quoted", "DSN=a b", "a #b", `DSN="a #b"`},
		{"a bare spaced value gaining an edge space is quoted", "DSN=a b", "a b ", `DSN="a b "`},
		{"a bare spaced value gaining a newline is quoted", "DSN=a b", "a\nb", "DSN=\"a\nb\""},
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
	for _, value := range []string{`say "hi"`, `a"b`, `"lead`, `it's`, `pa$w"rd`, `it's"fine`} {
		t.Run(value, func(t *testing.T) {
			lines, changed := Set(SetParams{Lines: Parse([]byte("A=1\n")), Key: "A", Value: value})
			require.True(t, changed)

			assert.Equal(t, value, valueOf(t, Render(lines), "A"))
			_, again := Set(SetParams{Lines: Parse(Render(lines)), Key: "A", Value: value})
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
	cases := map[string]string{
		"\ufeff":     "\ufeffK=V\n",
		"\ufeff\n\n": "\ufeffK=V\n\n\n",
		"\ufeff\r\n": "\ufeffK=V\r\n\r\n",
	}
	for content, want := range cases {
		t.Run(content, func(t *testing.T) {
			lines, _ := Set(SetParams{Lines: Parse([]byte(content)), Key: "K", Value: "V"})

			assert.Equal(t, want, string(Render(lines)))
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
