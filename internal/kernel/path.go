package kernel

import (
	"fmt"
	"reflect"
	"strings"
)

// A field's path is its JSON name, dotted through nested structs
// ("target.from"): the same string in a flag error, the schema and the form.

func Get[Req any](req Req, path string) (Value, error) {
	field, err := fieldAt(reflect.ValueOf(req), path)
	if err != nil {
		return Value{}, err
	}
	return valueOf(field, path)
}

// Set returns a copy of req with path set to value.
func Set[Req any](req Req, path string, value Value) (Req, error) {
	copied := reflect.New(reflect.TypeOf(req)).Elem()
	copied.Set(reflect.ValueOf(req))
	field, err := fieldAt(copied, path)
	if err != nil {
		return req, err
	}
	if err := assign(field, value, path); err != nil {
		return req, err
	}
	out, ok := copied.Interface().(Req)
	if !ok {
		return req, fmt.Errorf("kernel: set %s: expected %T, got %T", path, req, copied.Interface())
	}
	return out, nil
}

// Paths lists every leaf field of Req, nested structs walked through.
func Paths[Req any]() []string {
	var req Req
	return leafPaths(reflect.TypeOf(req), "")
}

func IsZero(value Value) bool {
	return value.Text == "" && !value.Bool && len(value.List) == 0 && len(value.Decisions) == 0
}

func leafPaths(typ reflect.Type, prefix string) []string {
	if typ == nil || typ.Kind() != reflect.Struct {
		return nil
	}
	var paths []string
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, ok := jsonName(field)
		if !ok {
			continue
		}
		path := prefix + name
		if field.Type.Kind() == reflect.Struct {
			paths = append(paths, leafPaths(field.Type, path+".")...)
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

func fieldAt(root reflect.Value, path string) (reflect.Value, error) {
	current := root
	for _, segment := range strings.Split(path, ".") {
		if current.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("kernel: path %q: %s is not a struct", path, current.Type())
		}
		next, found := childNamed(current, segment)
		if !found {
			return reflect.Value{}, fmt.Errorf("kernel: path %q: no field %q in %s", path, segment, current.Type())
		}
		current = next
	}
	return current, nil
}

func childNamed(parent reflect.Value, segment string) (reflect.Value, bool) {
	for i := range parent.NumField() {
		name, ok := jsonName(parent.Type().Field(i))
		if ok && name == segment {
			return parent.Field(i), true
		}
	}
	return reflect.Value{}, false
}

func jsonName(field reflect.StructField) (string, bool) {
	if !field.IsExported() {
		return "", false
	}
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "-" {
		return "", false
	}
	if name == "" {
		return field.Name, true
	}
	return name, true
}

func valueOf(field reflect.Value, path string) (Value, error) {
	switch {
	case field.Kind() == reflect.String:
		return Value{Text: field.String()}, nil
	case field.Kind() == reflect.Bool:
		return Value{Bool: field.Bool()}, nil
	case isStringSlice(field.Type()):
		return Value{List: stringsOf(field)}, nil
	case isStringMap(field.Type()):
		return Value{Decisions: decisionsOf(field)}, nil
	}
	return Value{}, fmt.Errorf("kernel: path %q: unsupported type %s", path, field.Type())
}

func assign(field reflect.Value, value Value, path string) error {
	switch {
	case field.Kind() == reflect.String:
		field.SetString(value.Text)
	case field.Kind() == reflect.Bool:
		field.SetBool(value.Bool)
	case isStringSlice(field.Type()):
		field.Set(sliceOf(field.Type(), value.List))
	case isStringMap(field.Type()):
		field.Set(mapOf(field.Type(), value.Decisions))
	default:
		return fmt.Errorf("kernel: path %q: unsupported type %s", path, field.Type())
	}
	return nil
}

func isStringSlice(typ reflect.Type) bool {
	return typ.Kind() == reflect.Slice && typ.Elem().Kind() == reflect.String
}

func isStringMap(typ reflect.Type) bool {
	return typ.Kind() == reflect.Map && typ.Key().Kind() == reflect.String && typ.Elem().Kind() == reflect.String
}

func stringsOf(field reflect.Value) []string {
	if field.Len() == 0 {
		return nil
	}
	out := make([]string, field.Len())
	for i := range field.Len() {
		out[i] = field.Index(i).String()
	}
	return out
}

func decisionsOf(field reflect.Value) map[string]string {
	if field.Len() == 0 {
		return nil
	}
	out := make(map[string]string, field.Len())
	iter := field.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().String()
	}
	return out
}

func sliceOf(typ reflect.Type, items []string) reflect.Value {
	if items == nil {
		return reflect.Zero(typ)
	}
	out := reflect.MakeSlice(typ, len(items), len(items))
	for i, item := range items {
		out.Index(i).SetString(item)
	}
	return out
}

func mapOf(typ reflect.Type, decisions map[string]string) reflect.Value {
	if decisions == nil {
		return reflect.Zero(typ)
	}
	out := reflect.MakeMapWithSize(typ, len(decisions))
	for key, decision := range decisions {
		out.SetMapIndex(reflect.ValueOf(key).Convert(typ.Key()), reflect.ValueOf(decision).Convert(typ.Elem()))
	}
	return out
}
