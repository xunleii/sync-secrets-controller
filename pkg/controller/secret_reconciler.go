package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SecretReconciler struct{ *Context }

func (r *SecretReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	secret := corev1.Secret{}
	klog.Infof("reconcile %T %s", secret, req)

	err := r.client.Get(r.Context, req.NamespacedName, &secret)
	if errors.IsNotFound(err) {
		klog.Errorf("failed to fetch %T %s: %s", secret, req.NamespacedName, err)
		klog.V(5).Infof("this error occurs when the secret is deleted")

		secret := r.registry.SecretWithName(req.NamespacedName)
		if secret == nil {
			return reconcile.Result{}, nil
		}

		if err := r.registry.UnregisterSecret(secret.UID); err != nil {
			klog.Errorf("failed to remove %T %s from registry: %s", secret, req, err)
		}
		return reconcile.Result{}, nil
	} else if err != nil {
		klog.Errorf("failed to fetch %T %s: %s... retry after %s", secret, req, err, requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}

	if len(secret.OwnerReferences) > 0 {
		klog.V(5).Infof("ignore %T %s: secret already owned by someone", secret, req)
		return reconcile.Result{}, nil
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
