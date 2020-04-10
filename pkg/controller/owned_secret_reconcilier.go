package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ownedSecretReconcilier struct{ *Context }

func (r *ownedSecretReconcilier) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	owned := corev1.Secret{}
	klog.Infof("reconcile owned %T %s", owned, req)

	owner := corev1.Secret{}
	ownerID := r.registry.SecretWithOwnedSecretName(req.NamespacedName)
	if ownerID == nil {
		klog.V(3).Infof("reconcile new owned %T %s", owner, req)

		err := r.client.Get(r.Context, req.NamespacedName, &owned)
		if errors.IsNotFound(err) {
			klog.Warningf("reconcile new owned %T %s, but not found... ignore reconcilation", owned, req)
			return reconcile.Result{}, nil
		} else if err != nil {
			klog.Errorf("failed to fetch %T %s: %s... retry after %s", owned, req, err, requeueAfter)
			return reconcile.Result{RequeueAfter: requeueAfter}, err
		}

		if len(owned.OwnerReferences) == 0 {
			klog.Warningf("reconcile owned %T without owner... ignore reconciliation", owned)
			return reconcile.Result{}, nil
		}

		ownerID := r.registry.SecretWithUID(owned.OwnerReferences[0].UID)
		if ownerID == nil {
			klog.Errorf("owner of %T %s (%s) is not registered... ignore reconciliation", owned, req, owned.OwnerReferences[0].UID)
			return reconcile.Result{}, nil
		}

		err = r.client.Get(r.Context, ownerID.NamespacedName, &owner)
		if err != nil {
			klog.Errorf("failed to fetch %T %s: %s... retry after %s", owned, req, err, requeueAfter)
			return reconcile.Result{RequeueAfter: requeueAfter}, err
		}

		klog.V(5).Infof("register new owned %T %s", owned, req)
		err = r.registry.RegisterOwnedSecret(ownerID.UID, req.NamespacedName)
		if err != nil {
			klog.Errorf("failed to register %T %s... ignore reconciliation", owned, req)
			return reconcile.Result{}, nil
		}
	} else {
		err := r.client.Get(r.Context, ownerID.NamespacedName, &owner)
		if err != nil {
			klog.Errorf("failed to fetch %T %s: %s... retry after %s", owned, req, err, requeueAfter)
			return reconcile.Result{RequeueAfter: requeueAfter}, err
		}
	}

	err := SynchronizeSecret(r.Context, owner)
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
