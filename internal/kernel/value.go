package kernel

import (
	"bytes"
	"encoding/json"
	"fmt"
)

//go-sumtype:decl Value

type Value interface {
	value()
}

type Text string

type Bool bool

type List []string

type Decisions map[string]string

func (Text) value()      {}
func (Bool) value()      {}
func (List) value()      {}
func (Decisions) value() {}

func IsZero(value Value) bool {
	switch v := value.(type) {
	case nil:
		return true
	case Text:
		return v == ""
	case Bool:
		return !bool(v)
	case List:
		return len(v) == 0
	case Decisions:
		return len(v) == 0
	}
	return true
}

func decodeValue(raw json.RawMessage) (Value, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	switch trimmed[0] {
	case '"':
		return decodeAs[Text](trimmed)
	case 't', 'f':
		return decodeAs[Bool](trimmed)
	case '[':
		return decodeAs[List](trimmed)
	case '{':
		return decodeAs[Decisions](trimmed)
	}
	return nil, fmt.Errorf("kernel: %s is not a value", trimmed)
}

func decodeAs[V Value](raw []byte) (Value, error) {
	var value V
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}
