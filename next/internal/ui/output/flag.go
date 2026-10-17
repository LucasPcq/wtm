package output

import "strings"

type Flags map[string]string

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
