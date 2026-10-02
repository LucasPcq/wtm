package infra

type Paths struct{ Root string }

func GlobalDir() (string, error) { return "", nil }

func Toplevel(dir string) (string, error) { return dir, nil }
