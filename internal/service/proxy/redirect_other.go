//go:build !darwin

package proxy

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
)

type unsupported struct{}

func NewRedirector(_ RedirectorParams) Redirector { return unsupported{} }

func (unsupported) Plan() (Plan, error)          { return Plan{}, domain.ErrProxyRedirectUnsupported }
func (unsupported) Apply(context.Context) error  { return domain.ErrProxyRedirectUnsupported }
func (unsupported) Remove(context.Context) error { return domain.ErrProxyRedirectUnsupported }
func (unsupported) Inspect() domain.ProxyStatus  { return domain.ProxyStatus{} }
