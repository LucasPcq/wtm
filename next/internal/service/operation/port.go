package operation

import "github.com/LucasPcq/wtm/next/internal/domain"

// Locker takes the locks an operation names, before it reads anything.
type Locker interface {
	// Take records op as running and takes its locks; it fails with a
	// conflict error naming the holder when another operation holds one.
	Take(op domain.Operation) (Hold, error)
}

// Hold is the set of locks an operation holds until it releases them.
type Hold interface {
	// Grow takes those of locks not held yet and reports whether it took any.
	Grow(locks []domain.Lock) (bool, error)
	// Protected reports whether the locks are really held; false means the
	// lock directory could not be used and the operation runs unprotected.
	Protected() bool
	// Release gives the locks back and forgets the operation's record.
	Release()
}

// Publisher broadcasts an operation's events to the other processes listening.
type Publisher interface {
	// Open starts publishing for op and returns the function that publishes
	// each of its events.
	Open(op domain.Operation) domain.Report
}
