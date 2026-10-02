package method

type store struct{}

func (store) Create() error { return nil }

func run() error { return store{}.Create() }
