package env

func ApplyEnvSync() error { return nil }

func ApplyEnvPorts() error { return nil }

func sync() error { return ApplyEnvPorts() }
