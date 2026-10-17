package output

import "strings"

// Flags maps the path of a request's field to the flag that sets it, so a failure names what the user typed.
type Flags map[string]string

// Spell returns the flag that sets the field at path, keeping the index it carries (branches[1] → <branches>[1]), or path itself when no flag sets it.
func (f Flags) Spell(path string) string {
	name, index, indexed := strings.Cut(path, "[")
	flag, known := f[name]
	switch {
	case !known:
		return path
	case indexed:
		return flag + "[" + index
	}
	return flag
}
