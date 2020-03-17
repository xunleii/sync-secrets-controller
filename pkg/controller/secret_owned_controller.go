package controller

import (
	gocontext "context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconcileOwnedSecret struct {
	*context
	client client.Client
}

func (r *reconcileOwnedSecret) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	klog.Infof("Owned secret reconciliation on %s", req)

	secret := &corev1.Secret{}
	err := r.client.Get(gocontext.TODO(), req.NamespacedName, secret)
	if errors.IsNotFound(err) {
		klog.V(3).Infof("Failed to fetch %T %s: %s", secret, req.NamespacedName, err)
		klog.V(5).Infof("This error occurs when a secret is deleted")
		return reconcile.Result{}, nil
	} else if err != nil {
		klog.Errorf("Failed to fetch %T %s: %s... retry after %s", secret, req, err, requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}

	if len(secret.OwnerReferences) == 0 {
		return reconcile.Result{}, nil
	}

	ownerUid := secret.OwnerReferences[0].UID
	ownerRef, exists := r.owners.Load(ownerUid)
	if !exists {
		klog.Errorf("Unknown owner %s of %T %s... retry after %s", ownerUid, secret, req, requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}
	klog.V(4).Infof("%s is owner of %s", ownerRef, req)

	owner := &corev1.Secret{}
	err = r.client.Get(gocontext.TODO(), ownerRef.(types.NamespacedName), owner)
	if errors.IsNotFound(err) {
		klog.V(3).Infof("Secret owner %T %s doesn't exists... ignore", owner, ownerRef)
		return reconcile.Result{}, nil
	} else if err != nil {
		klog.Errorf("Failed to fetch %T %s: %s... ignore", owner, ownerRef, err)
		return reconcile.Result{}, nil
	}

	_ = copySecret(r.client, owner, req.NamespacedName)
	return reconcile.Result{}, nil
}

func (r *reconcileOwnedSecret) DeepCopy() *reconcileOwnedSecret {
	return &reconcileOwnedSecret{
		context: r.context.DeepCopy(),
		client:  r.client,
	}
}
