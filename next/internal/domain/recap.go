package domain

import "slices"

// A RecapRow is one line of a plan's recap, as every surface shows it: what it shows under a stable Key, its label, the field whose answer it shows if one does, the mark of that answer, and its values, in order.
type RecapRow struct {
	Key    string       `json:"key"`
	Label  string       `json:"label"`
	Path   string       `json:"path,omitempty"`
	Mark   Mark         `json:"mark,omitempty"`
	Values []RecapValue `json:"values"`
}

// A RecapValue is one value of a recap's row: the value, the stable Note that qualifies it with its params, and Text, both spelled as the command line prints them.
type RecapValue struct {
	Value  string `json:"value"`
	Note   string `json:"note,omitempty"`
	Params Params `json:"params,omitempty"`
	Text   string `json:"text"`
}

// RecapValues are values as a recap's values, each spelled as it is.
func RecapValues(values []string) []RecapValue {
	recapped := make([]RecapValue, 0, len(values))
	for _, value := range values {
		recapped = append(recapped, RecapValue{Value: value, Text: value})
	}

	return recapped
}

// A Mark says where the answer a recap's row shows comes from, or where it goes.
type Mark string

// The marks of an answer.
const (
	MarkRecalled Mark = "recalled" // answered from memory
	MarkRemember Mark = "remember" // to be remembered once applied
	MarkSaved    Mark = "saved"    // remembered by the apply
)

// Marks say which answers of a plan are marked, by the path of their field: those recalled from memory, and those remembered for the next runs with the mark that says so, MarkRemember before the apply, MarkSaved after it.
type Marks struct {
	Recalled   []string
	Remembered []string
	Mark       Mark
}

// Of is the mark of the answer to the field at path, empty when it has none.
func (m Marks) Of(path string) Mark {
	switch {
	case slices.Contains(m.Remembered, path):
		return m.Mark
	case slices.Contains(m.Recalled, path):
		return MarkRecalled
	}

	return ""
}
