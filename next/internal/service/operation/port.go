package operation

import "github.com/LucasPcq/wtm/next/internal/domain"

type Locker interface {
	Take(op domain.Operation) (Hold, error)
}

type Hold interface {
	Grow(locks []domain.Lock) (bool, error)
	Protected() bool
	Release()
}

type Publisher interface {
	Open(op domain.Operation) domain.Report
}
