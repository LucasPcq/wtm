package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	listSeparator = ", "
	pairFmt       = "%s " + domain.MoveArrowGlyph + " %s"
	jsonNull      = "null"
)

var yesNo = map[bool]string{true: "yes", false: "no"}

type Detail interface {
	Plan(detail json.RawMessage) []styles.PlanRow
	Item(w io.Writer, item kernel.Item[json.RawMessage])
}

type GenericDetail struct{}

func (GenericDetail) Plan(detail json.RawMessage) []styles.PlanRow {
	fields := fieldsOf(detail)
	rows := make([]styles.PlanRow, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, styles.PlanRow{Label: labelOfKey(field.key), Values: valuesOf(field.value)})
	}
	return rows
}

func (GenericDetail) Item(io.Writer, kernel.Item[json.RawMessage]) {}

type field struct {
	key   string
	value json.RawMessage
}

// A detail that is not an object is one field with no key.
func fieldsOf(detail json.RawMessage) []field {
	decoder := json.NewDecoder(bytes.NewReader(detail))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return []field{{value: detail}}
	}
	var fields []field
	for decoder.More() {
		next, ok := nextField(decoder)
		if !ok {
			return fields
		}
		fields = append(fields, next)
	}
	return fields
}

func nextField(decoder *json.Decoder) (field, bool) {
	token, err := decoder.Token()
	if err != nil {
		return field{}, false
	}
	key, ok := token.(string)
	if !ok {
		return field{}, false
	}
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return field{}, false
	}
	return field{key: key, value: value}, true
}

func valuesOf(value json.RawMessage) []string {
	trimmed := bytes.TrimSpace(value)
	if string(trimmed) == jsonNull {
		return []string{domain.SummaryNone}
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		return []string{text}
	}
	var flag bool
	if json.Unmarshal(trimmed, &flag) == nil {
		return []string{yesNo[flag]}
	}
	var list []string
	if json.Unmarshal(trimmed, &list) == nil {
		return orNone(list)
	}
	var pairs map[string]string
	if json.Unmarshal(trimmed, &pairs) == nil {
		shown := make([]string, 0, len(pairs))
		for _, key := range slices.Sorted(maps.Keys(pairs)) {
			shown = append(shown, fmt.Sprintf(pairFmt, key, pairs[key]))
		}
		return orNone(shown)
	}
	return []string{string(trimmed)}
}

func orNone(values []string) []string {
	if len(values) == 0 {
		return []string{domain.SummaryNone}
	}
	return values
}

func joined(values []string) string { return strings.Join(values, listSeparator) }

func labelOfKey(key string) string {
	var b strings.Builder
	for i, r := range key {
		switch {
		case r == '_' || r == '-':
			b.WriteRune(' ')
		case i == 0:
			b.WriteRune(unicode.ToUpper(r))
		case unicode.IsUpper(r):
			b.WriteRune(' ')
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
