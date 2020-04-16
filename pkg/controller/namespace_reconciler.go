package controller

import (
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NamespaceReconciler struct{ *Context }

func (n *NamespaceReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	if funk.ContainsString(n.IgnoredNamespaces, req.Name) {
		klog.V(3).Infof("namespace %s is ignored, ignore synchronization of %T %s", req.Name, corev1.Namespace{}, req.Name)
		return reconcile.Result{}, nil
	}

	reconciler := &SecretReconciler{n.Context}
	emptyResult := reconcile.Result{}

	secrets := n.registry.Secrets()
	klog.V(3).Infof("reconcile all synchronized secrets: %v", secrets)
	for _, namespacedName := range secrets {
		res, err := reconciler.Reconcile(reconcile.Request{NamespacedName: namespacedName})
		if err != nil || res != emptyResult {
			klog.Errorf("failed to reconcile %T %s", corev1.Namespace{}, req)
			return res, err
		}
	}

	return reconcile.Result{}, nil
}
