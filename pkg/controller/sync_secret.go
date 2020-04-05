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
	AnnotationError error
	RegistryError   error
	ClientError     error
)

// SynchronizeSecret duplicates the given secret on the target namespaces
func SynchronizeSecret(ctx *context, secret corev1.Secret) error {
	name := types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}
	if funk.ContainsString(ctx.ignoredNamespaces, secret.Namespace) {
		klog.V(3).Infof("namespace %s is ignored, ignore synchronization of %T %s", secret.Namespace, secret, name)
		return nil
	}


	onAllNamespaces, allExists := secret.Annotations[allNamespacesAnnotation]
	namespaceSelector, selectorExists := secret.Annotations[namespaceSelectorAnnotation]

	klog.V(3).Infof("manage annotation of %T %s", secret, name)
	var options []client.ListOption
	switch {
	case !allExists && !selectorExists:
		_ = ctx.registry.UnregisterSecret(secret.UID) //NOTE: ignore if secret not already registered
		klog.V(3).Infof("no annotation found on %T %s, ignore synchronization", secret, name)
		return nil
	case allExists && selectorExists:
		_ = ctx.registry.UnregisterSecret(secret.UID) //NOTE: ignore if secret not already registered
		return AnnotationError(fmt.Errorf("annotation '%s' and '%s' cannot be used together", allNamespacesAnnotation, namespaceSelectorAnnotation))
	case allExists:
		if strings.ToLower(onAllNamespaces) != "true" {
			_ = ctx.registry.UnregisterSecret(secret.UID) //NOTE: ignore if secret not already registered
			return AnnotationError(fmt.Errorf("'%s' is not 'true'", allNamespacesAnnotation))
		}
	case selectorExists:
		selector, err := labels.Parse(namespaceSelector)
		if err != nil {
			_ = ctx.registry.UnregisterSecret(secret.UID) //NOTE: ignore if secret not already registered
			return AnnotationError(fmt.Errorf("failed to parse '%s': %w", namespaceSelectorAnnotation, err))
		}
		options = append(options, client.MatchingLabelsSelector{Selector: selector})
	}

	klog.V(3).Infof("register %T %s internally", secret, name)
	if err := ctx.registry.RegisterSecret(name, secret.UID); err != nil {
		return RegistryError(err)
	}

	copy := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secret.Name,
			Labels:      secret.Labels,
			Annotations: secret.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: secret.APIVersion,
				Kind:       secret.Kind,
				Name:       secret.Name,
				UID:        secret.UID,
			}},
		},
		Data: secret.Data,
		Type: secret.Type,
	}
	delete(copy.Annotations, allNamespacesAnnotation)
	delete(copy.Annotations, namespaceSelectorAnnotation)

	klog.V(3).Infof("list all namespaces", secret, name)
	ignoredNamespaces := append(ctx.ignoredNamespaces, secret.Namespace)
	namespaces := &corev1.NamespaceList{}
	if err := ctx.client.List(ctx, namespaces, options...); err != nil {
		return ClientError(fmt.Errorf("failed to list namespaces: %w", err))
	}

	klog.V(0).Infof("synchronize %T %s on all namespaces except %v", secret, name, ignoredNamespaces)
	for _, namespace := range namespaces.Items {
		if funk.ContainsString(ignoredNamespaces, namespace.Name) {
			continue
		}

		klog.V(2).Infof("synchronize %T %s on %s", secret, name, namespace.Name)
		ownedName := types.NamespacedName{Namespace: namespace.Name, Name: secret.Name}
		ownedSecret := &corev1.Secret{}
		copy.Namespace = namespace.Name

		klog.V(3).Infof("fetch %T %s", secret, name)
		err := ctx.client.Get(ctx, ownedName, ownedSecret)
		if errors.IsNotFound(err) {
			klog.V(3).Infof("create %T %s", secret, name)
			if err := ctx.client.Create(ctx, &copy); err != nil {
				return ClientError(fmt.Errorf("failed to create %T %s: %w", ownedSecret, ownedName, err))
			}
			return nil
		} else if err != nil {
			return ClientError(fmt.Errorf("failed to fetch %T %s: %w", ownedSecret, ownedName, err))
		}

		if !(len(secret.OwnerReferences) == 1 && secret.OwnerReferences[0].UID == secret.UID) {
			klog.V(0).Infof("secret %s not owned by %T %s... ignore", ownedName, secret, name)
			continue
		}

		klog.V(3).Infof("update %T %s", secret, name)
		if err = ctx.client.Update(ctx, &copy); err != nil {
			return ClientError(fmt.Errorf("failed to update %T %s: %w", ownedSecret, ownedName, err))
		}
	}
	return nil
}
