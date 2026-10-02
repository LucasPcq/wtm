package runjobs

import "github.com/LucasPcq/wtm/internal/service/worktree"

type index struct{}

func (index) Create() error { return nil }

func start() error {
	if err := (index{}).Create(); err != nil {
		return err
	}
	_, err := worktree.EnsureOrdinal() // want `worktree\.EnsureOrdinal is called from service/`
	return err
}
