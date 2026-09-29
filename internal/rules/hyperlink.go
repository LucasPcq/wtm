package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

var urlPattern = regexp.MustCompile(domain.URLPattern)

// URLSpan is one address found in a line, by byte offset.
type URLSpan struct {
	URL        string
	Start, End int
}

// URLSpans finds every address in text that can be followed whole: one cut
// short by an ellipsis is the start of an address, and opening it would land
// somewhere else.
func URLSpans(text string) []URLSpan {
	var spans []URLSpan
	for _, match := range urlPattern.FindAllStringIndex(text, -1) {
		if strings.HasPrefix(text[match[1]:], domain.Ellipsis) {
			continue
		}
		spans = append(spans, URLSpan{URL: text[match[0]:match[1]], Start: match[0], End: match[1]})
	}
	return spans
}

// LinkURLs wraps every address in text in an OSC-8 sequence, so a terminal
// makes it clickable. The caller decides whether the stream is one: a pipe, a
// JSON document or a `$(…)` must never receive the escape.
func LinkURLs(text string) string {
	if strings.Contains(text, domain.HyperlinkOpen) {
		return text
	}
	spans := URLSpans(text)
	if len(spans) == 0 {
		return text
	}
	var b strings.Builder
	last := 0
	for _, span := range spans {
		b.WriteString(text[last:span.Start])
		b.WriteString(fmt.Sprintf(domain.HyperlinkFmt, span.URL, span.URL))
		last = span.End
	}
	b.WriteString(text[last:])
	return b.String()
}
