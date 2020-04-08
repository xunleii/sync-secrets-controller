package controller

import (
	gocontext "context"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/xunleii/sync-secrets-operator/pkg/registry"
)

type (
	// Context contains all required information used by the controller to
	// synchronize secrets, like kubernetes client or controller configuration.
	Context struct {
		gocontext.Context
		IgnoredNamespaces []string

		client   client.Client
		registry registry.Registry

		owners *sync.Map //TODO: only for legacy ... remove it
	}
)

// NewContext creates a new context instance.
func NewContext(ctx gocontext.Context, client client.Client) *Context {
	return &Context{
		Context:  ctx,
		client:   client,
		registry: *registry.New(),

		owners: &sync.Map{},
	}
}
