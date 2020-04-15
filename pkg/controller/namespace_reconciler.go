package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NamespaceReconciler struct{ *Context }

func (n *NamespaceReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

