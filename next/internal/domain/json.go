package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// encoded is v as JSON without HTML escaping: a flag such as <branches> stays readable.
func encoded(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// discriminated is v, a struct, as a JSON object whose first member is key set to name: the discriminant of the variant of a sealed interface v is.
func discriminated(key, name string, v any) ([]byte, error) {
	body, err := encoded(v)
	if err != nil {
		return nil, err
	}
	if len(body) < 2 || body[0] != '{' {
		return nil, fmt.Errorf("%s %q: expected a JSON object, got %s", key, name, body)
	}

	head, _ := encoded(map[string]string{key: name})
	if string(body) == "{}" {
		return head, nil
	}
	return append(append(head[:len(head)-1], ','), body[1:]...), nil
}
