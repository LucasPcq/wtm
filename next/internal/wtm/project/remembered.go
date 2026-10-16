package project

const (
	RememberEnvStrategy  = "env_strategy"
	RememberIsolation    = "isolation"
	RememberSourceUpdate = "source_update"

	SourceUpdateFastForward = "ff"
	SourceUpdateKeep        = "keep"
)

func RememberableValues(id string) ([]string, bool) {
	switch id {
	case RememberEnvStrategy:
		return []string{string(EnvExample), string(EnvMain), string(EnvParent)}, true
	case RememberIsolation:
		return []string{"isolated", "verbatim"}, true
	case RememberSourceUpdate:
		return []string{SourceUpdateFastForward, SourceUpdateKeep}, true
	}
	return nil, false
}

func RememberIDs() []string {
	return []string{RememberEnvStrategy, RememberIsolation, RememberSourceUpdate}
}
