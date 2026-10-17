// Package env decides what the .env files of a worktree hold — their sync with their source, the ports, the keys wtm owns — and has store write it.
package env

import "github.com/LucasPcq/wtm/internal/infra/fs"

func fileExists(path string) bool {
	return fs.FileExists(path)
}
