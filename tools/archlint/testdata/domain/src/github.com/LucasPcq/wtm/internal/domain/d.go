package domain

type Worktree struct{ Branch string }

const Main = "main"

func Helper() string { return Main } // want `internal/domain declares "Helper": this layer holds types, errors and constants only`
