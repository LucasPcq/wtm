package kernel

import (
	"fmt"
	"reflect"
	"strings"
)

// A field's path is its JSON name, dotted through nested structs
// ("target.from"); an entry of a list or of decisions is path[index] or
// path[key]. The same string names the field in a flag error, the schema and
// the form.

// Get reads a field as the Value its Go type holds: a string as Text, a bool
// as Bool, a []string as List, a map[string]string as Decisions.
func Get[Req any](req Req, path string) (Value, error) {
	field, err := fieldAt(reflect.ValueOf(req), path)
	if err != nil {
		return nil, err
	}
	return valueOf(field, path)
}

// Set returns a copy of req with path set to value; nil empties the field. A
// value of another shape than the field's is an error.
func Set[Req any](req Req, path string, value Value) (Req, error) {
	copied := reflect.New(reflect.TypeOf(req)).Elem()
	copied.Set(reflect.ValueOf(req))
	field, err := fieldAt(copied, path)
	if err != nil {
		return req, err
	}
	if err := assign(field, assignment{path: path, value: value}); err != nil {
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

// IsZero is true for no value and for an empty one.
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

type PathIn struct {
	Path   string
	Parent string
}

// Within says whether Path is Parent itself, one of its entries
// (parent[2]) or a field nested in it (parent.from).
func Within(in PathIn) bool {
	rest, found := strings.CutPrefix(in.Path, in.Parent)
	return found && (rest == "" || strings.HasPrefix(rest, "[") || strings.HasPrefix(rest, "."))
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
		return Text(field.String()), nil
	case field.Kind() == reflect.Bool:
		return Bool(field.Bool()), nil
	case isStringSlice(field.Type()):
		return List(stringsOf(field)), nil
	case isStringMap(field.Type()):
		return Decisions(decisionsOf(field)), nil
	}
	return nil, fmt.Errorf("kernel: path %q: unsupported type %s", path, field.Type())
}

type assignment struct {
	path  string
	value Value
}

func assign(field reflect.Value, to assignment) error {
	if to.value == nil {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}
	switch v := to.value.(type) {
	case Text:
		return assignIf(field.Kind() == reflect.String, to, func() { field.SetString(string(v)) })
	case Bool:
		return assignIf(field.Kind() == reflect.Bool, to, func() { field.SetBool(bool(v)) })
	case List:
		return assignIf(isStringSlice(field.Type()), to, func() { field.Set(sliceOf(field.Type(), v)) })
	case Decisions:
		return assignIf(isStringMap(field.Type()), to, func() { field.Set(mapOf(field.Type(), v)) })
	}
	return fmt.Errorf("kernel: path %q: unknown value %T", to.path, to.value)
}

func assignIf(fits bool, to assignment, set func()) error {
	if !fits {
		return fmt.Errorf("kernel: path %q: a %T does not fit this field", to.path, to.value)
	}
	set()
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

func sliceOf(typ reflect.Type, items List) reflect.Value {
	if items == nil {
		return reflect.Zero(typ)
	}
	out := reflect.MakeSlice(typ, len(items), len(items))
	for i, item := range items {
		out.Index(i).SetString(item)
	}
	return out
}

func mapOf(typ reflect.Type, decisions Decisions) reflect.Value {
	if decisions == nil {
		return reflect.Zero(typ)
	}
	out := reflect.MakeMapWithSize(typ, len(decisions))
	for key, decision := range decisions {
		out.SetMapIndex(reflect.ValueOf(key).Convert(typ.Key()), reflect.ValueOf(decision).Convert(typ.Elem()))
	}
	return out
}
