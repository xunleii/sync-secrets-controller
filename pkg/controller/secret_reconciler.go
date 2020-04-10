package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type secretReconciler struct{ *Context }

func (r *secretReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	secret := corev1.Secret{}
	klog.Infof("reconcile %T %s", secret, req)

	err := r.client.Get(r.Context, req.NamespacedName, &secret)
	if errors.IsNotFound(err) {
		klog.Errorf("failed to fetch %T %s: %s", secret, req.NamespacedName, err)
		klog.V(5).Infof("this error occurs when a secret is deleted")

		secret := r.registry.SecretWithOwnedSecretName(req.NamespacedName)
		if secret == nil {
			return reconcile.Result{}, nil
		}

		if err := r.registry.UnregisterSecret(secret.UID); err != nil {
			klog.Errorf("remove %T %s from owner table", secret, req)
		}
		return reconcile.Result{}, nil
	} else if err != nil {
		klog.Errorf("failed to fetch %T %s: %s... retry after %s", secret, req, err, requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}

	err = SynchronizeSecret(r.Context, secret)
	if err == nil {
		return reconcile.Result{}, nil
	}
	klog.Errorf("failed to synchronize %T %s: %s", secret, req.NamespacedName, err)

	switch err.(type) {
	case AnnotationError:
		return reconcile.Result{}, nil
	case RegistryError:
		return reconcile.Result{}, nil
	default:
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}
}
