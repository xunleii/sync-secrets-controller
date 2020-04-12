package controller

import (
	"fmt"
	"strings"

	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	NoAnnotationError struct{ error }
	AnnotationError   struct{ error }
	RegistryError     struct{ error }
	ClientError       struct{ error }
)

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

	namespaces, err := listNamespaces(ctx, secret)
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
	delete(template.Annotations, AllNamespacesAnnotation)
	delete(template.Annotations, NamespaceSelectorAnnotation)

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

/*
Synchronize an owned secret
	- Check if secret must be synced (or just deleted)
	- Get namespace
	- Apply owner secret
*/
// SynchronizeOwnedSecret duplicates the given secret in the given namespace.
func SynchronizeOwnedSecret(ctx *Context, secret corev1.Secret, namespace string) error {
	name := types.NamespacedName{Namespace: namespace, Name: secret.Name}
	template := secret.DeepCopy()
	template.ObjectMeta = metav1.ObjectMeta{
		Name:        template.Name,
		Namespace:   namespace,
		Labels:      template.Labels,
		Annotations: template.Annotations,
		OwnerReferences: []metav1.OwnerReference{
			{APIVersion: "v1", Kind: "Secret", Name: secret.Name, UID: secret.UID},
		},
	}
	delete(template.Annotations, AllNamespacesAnnotation)
	delete(template.Annotations, NamespaceSelectorAnnotation)

	klog.V(3).Infof("update %T %s", secret, name)
	err := ctx.client.Update(ctx, template)
	if errors.IsNotFound(err) {
		if err = ctx.client.Create(ctx, template); err != nil {
			return ClientError{fmt.Errorf("failed to create %T %s: %w", secret, name, err)}
		}
	} else if err != nil {
		return ClientError{fmt.Errorf("failed to update %T %s: %w", secret, name, err)}
	}
	return nil
}

// listNamespaces lists all namespaces based on the secret annotations.
func listNamespaces(ctx *Context, secret corev1.Secret) ([]string, error) {
	var options []client.ListOption

	allNamespaces, hasAllNamespace := secret.Annotations[AllNamespacesAnnotation]
	namespaceSelector, hasNamespaceSelector := secret.Annotations[NamespaceSelectorAnnotation]

	var err error
	switch {
	case hasAllNamespace && hasNamespaceSelector:
		err = AnnotationError{fmt.Errorf("annotation '%s' and '%s' cannot be used together", AllNamespacesAnnotation, NamespaceSelectorAnnotation)}
	case hasAllNamespace:
		if strings.ToLower(allNamespaces) != "true" {
			err = AnnotationError{fmt.Errorf("'%s' is not 'true'", AllNamespacesAnnotation)}
		}
	case hasNamespaceSelector:
		var selector labels.Selector
		selector, err = labels.Parse(namespaceSelector)
		if err != nil {
			err = AnnotationError{fmt.Errorf("failed to parse '%s': %w", NamespaceSelectorAnnotation, err)}
		} else {
			options = append(options, client.MatchingLabelsSelector{Selector: selector})
		}
	default:
		err = NoAnnotationError{fmt.Errorf("no annotation found, ignore synchronization")}
	}

	if err != nil {
		return nil, err
	}

	namespaceObjects := &corev1.NamespaceList{}
	if err := ctx.client.List(ctx, namespaceObjects, options...); err != nil {
		return nil, ClientError{fmt.Errorf("failed to list namespaces: %w", err)}
	}

	ignoredNamespace := map[string]struct{}{}
	for _, namespace := range append(ctx.IgnoredNamespaces, secret.Namespace) {
		ignoredNamespace[namespace] = struct{}{}
	}

	namespaces := make([]string, 0, len(namespaceObjects.Items))
	for _, namespace := range namespaceObjects.Items {
		if _, exists := ignoredNamespace[namespace.Name]; !exists {
			namespaces = append(namespaces, namespace.Name)
		}
	}
	return namespaces, nil
}
