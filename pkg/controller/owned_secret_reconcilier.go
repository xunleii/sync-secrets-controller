package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OwnedSecretReconcilier struct{ *Context }

func (r *OwnedSecretReconcilier) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	owned := corev1.Secret{}
	klog.Infof("reconcile owned %T %s", owned, req)

	ownerID := r.registry.SecretWithOwnedSecretName(req.NamespacedName)
	if ownerID == nil {
		klog.V(3).Infof("owned %T %s in termination mode... ignore reconciliation", owned, req)
		return reconcile.Result{}, nil
	}

	owner := corev1.Secret{}
	err := r.client.Get(r.Context, ownerID.NamespacedName, &owner)
	if err != nil {
		klog.Errorf("failed to fetch %T %s: %s... retry after %s", owned, req, err, requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}

	err = SynchronizeOwnedSecret(r.Context, owner, req.Namespace)
	if err == nil {
		return reconcile.Result{}, nil
	}
	klog.Errorf("failed to synchronize %T %s: %s", owned, req, err)

	switch err.(type) {
	case AnnotationError:
		return reconcile.Result{}, nil
	case RegistryError:
		return reconcile.Result{}, nil
	default:
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}
}
