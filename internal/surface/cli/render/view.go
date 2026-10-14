package render

import (
	"encoding/json"
	"io"
)

func View(w io.Writer, command string, view json.RawMessage) {
	Announce(w, command, rowsOf(view))
}

func rowsOf(detail json.RawMessage) []AnnounceItem {
	fields := fieldsOf(detail)
	rows := make([]AnnounceItem, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, AnnounceItem{Label: field.key, Value: joined(valuesOf(field.value))})
	}
	return rows
}
