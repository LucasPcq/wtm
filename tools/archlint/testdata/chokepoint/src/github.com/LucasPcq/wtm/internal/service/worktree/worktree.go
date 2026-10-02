package worktree

func Create() error { return nil }

func SetIsolation() error { return nil }

func EnsureOrdinal() (int, error) { return 0, nil }

func List() error { return nil }

func BranchEnv() (int, error) { return EnsureOrdinal() }
