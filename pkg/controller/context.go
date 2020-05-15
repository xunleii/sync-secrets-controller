package controller

import (
	gocontext "context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/xunleii/sync-secrets-controller/pkg/registry"
)

type (
	// Context contains all required information used by the controller to
	// synchronize secrets, like kubernetes client or controller configuration.
	Context struct {
		gocontext.Context
		IgnoredNamespaces []string

		client   client.Client
		registry *registry.Registry
	}
)

// NewContext creates a new context instance.
func NewContext(ctx gocontext.Context, client client.Client) *Context {
	return &Context{
		Context:  ctx,
		client:   client,
		registry: registry.New(),
	}
}

// NewTestContext creates a new context instance for testing purpose.
func NewTestContext(ctx gocontext.Context, client client.Client, registry *registry.Registry) *Context {
	return &Context{
		Context:  ctx,
		client:   client,
		registry: registry,
	}
}
