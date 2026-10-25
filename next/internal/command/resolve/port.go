package resolve

import "github.com/LucasPcq/wtm/next/internal/service/repository"

// Git resolves a path to where it really is, and whether something is there.
type Git interface {
	repository.Locator
}
