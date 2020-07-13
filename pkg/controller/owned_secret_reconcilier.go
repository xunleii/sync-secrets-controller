package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// SynchronizeOwnedSecret duplicates the given secret in the given namespace.
func SynchronizeOwnedSecret(ctx *Context, ownerSecret corev1.Secret, namespace string) error {
	name := types.NamespacedName{Namespace: namespace, Name: ownerSecret.Name}
	template := ownerSecret.DeepCopy()
	template.ObjectMeta = metav1.ObjectMeta{
		Name:        template.Name,
		Namespace:   namespace,
		Labels:      template.Labels,
		Annotations: template.Annotations,
		OwnerReferences: []metav1.OwnerReference{
			{APIVersion: "v1", Kind: "Secret", Name: ownerSecret.Name, UID: ownerSecret.UID},
		},
	}
	template = assignOriginMetadata(template, &ownerSecret)
	template = excludeProtectedMetadata(ctx, template)

	secret := corev1.Secret{}
	klog.V(3).Infof("fetch %T %s", secret, name)
	err := ctx.client.Get(ctx, name, &secret)
	if errors.IsNotFound(err) {
		klog.V(3).Infof("%T %s not found, create it", secret, name)
		if err = ctx.client.Create(ctx, template); err != nil {
			return ClientError{fmt.Errorf("failed to create %T %s: %w", secret, name, err)}
		}
		return nil
	} else if err != nil {
		return ClientError{fmt.Errorf("failed to fetch %T %s: %w", secret, name, err)}
	}

	secret.SetName(template.GetName())
	secret.SetNamespace(namespace)
	secret.SetLabels(template.GetLabels())
	secret.SetAnnotations(template.GetAnnotations())
	secret.SetOwnerReferences(template.GetOwnerReferences())
	secret.StringData = template.StringData
	secret.Data = template.Data

	klog.V(3).Infof("update %T %s", ownerSecret, name)
	if err = ctx.client.Update(ctx, &secret); err != nil {
		return ClientError{fmt.Errorf("failed to update %T %s: %w", ownerSecret, name, err)}
	}
	return nil
}
