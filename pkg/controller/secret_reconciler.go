package controller

import (
	"fmt"

	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// SynchronizeSecret duplicates the given secret on namespaces matching with its annotation.
func SynchronizeSecret(ctx *Context, secret corev1.Secret) error {
	name := types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}
	if funk.ContainsString(ctx.IgnoredNamespaces, secret.Namespace) {
		klog.V(3).Infof("namespace %s is ignored, ignore synchronization of %T %s", secret.Namespace, secret, name)
		return nil
	}

	ownedSecrets := ctx.registry.OwnedSecretsWithUID(secret.UID)
	syncedNamespaces := make([]string, len(ownedSecrets))
	for i, owned := range ownedSecrets {
		syncedNamespaces[i] = owned.Namespace
	}

	namespaces, err := listNamespacesFromAnnotations(ctx, secret)
	if _, noAnnotation := err.(NoAnnotationError); noAnnotation && len(ownedSecrets) == 0 {
		// NOTE: if secret doesn't have annotation and doesn't have owned secret,
		//       this is an unmanaged secret
		return nil
	}

	unsyncedNamespaces := funk.LeftJoinString(syncedNamespaces, namespaces)

	template := secret.DeepCopy()
	template.ObjectMeta = metav1.ObjectMeta{
		Name:        template.Name,
		Labels:      template.Labels,
		Annotations: template.Annotations,
		OwnerReferences: []metav1.OwnerReference{
			{APIVersion: "v1", Kind: "Secret", Name: secret.Name, UID: secret.UID},
		},
	}
	template = excludeProtectedMetadata(ctx, template)

	for _, namespace := range unsyncedNamespaces {
		secret := template.DeepCopy()
		secret.Namespace = namespace
		klog.V(3).Infof("delete %T %s/%s", secret, namespace, secret.Name)
		_ = ctx.registry.UnregisterOwnedSecret(types.NamespacedName{Namespace: namespace, Name: secret.Name})
		if err := ctx.client.Delete(ctx, secret); err != nil {
			return ClientError{error: err}
		}
	}

	// NOTE: if an annotation error occurs, we don't need to create or update
	//       owned secrets.
	if err != nil {
		return err
	}

	if err = ctx.registry.RegisterSecret(name, secret.UID); err != nil {
		return RegistryError{error: err}
	}

	owner := secret
	ownerName := name

	for _, namespace := range namespaces {
		secret := &corev1.Secret{}
		name := types.NamespacedName{Namespace: namespace, Name: name.Name}

		klog.V(3).Infof("fetch %T %s", secret, name)
		err := ctx.client.Get(ctx, name, secret)
		if errors.IsNotFound(err) {
			secret := template.DeepCopy()
			secret.Namespace = namespace

			klog.V(3).Infof("%T %s not found, create it", secret, name)
			if err := ctx.client.Create(ctx, secret); err != nil {
				return ClientError{fmt.Errorf("failed to create %T %s: %w", secret, name, err)}
			}
			_ = ctx.registry.RegisterOwnedSecret(owner.UID, name)
			continue
		} else if err != nil {
			return ClientError{fmt.Errorf("failed to fetch %T %s: %w", secret, name, err)}
		}

		if len(secret.OwnerReferences) == 0 || secret.OwnerReferences[0].UID != owner.UID {
			klog.V(0).Infof("secret %s not owned by %T %s... ignore", name, secret, ownerName)
			continue
		}

		secret = template.DeepCopy()
		secret.Namespace = namespace

		klog.V(3).Infof("update %T %s", secret, name)
		if err = ctx.client.Update(ctx, secret); err != nil {
			return ClientError{fmt.Errorf("failed to update %T %s: %w", secret, name, err)}
		}
		_ = ctx.registry.RegisterOwnedSecret(owner.UID, name)
	}
	return nil
}
