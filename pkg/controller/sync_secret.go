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
	AnnotationError struct{ error }
	RegistryError   struct{ error }
	ClientError     struct{ error }
)

// SynchronizeSecret duplicates the given secret on the target namespaces
// TODO: Need to remove all owned secret when update
func SynchronizeSecret(ctx *Context, secret corev1.Secret, targetNamespaces ...string) error {
	name := types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}
	if funk.ContainsString(ctx.IgnoredNamespaces, secret.Namespace) {
		klog.V(3).Infof("namespace %s is ignored, ignore synchronization of %T %s", secret.Namespace, secret, name)
		return nil
	}

	onAllNamespaces, allExists := secret.Annotations[AllNamespacesAnnotation]
	namespaceSelector, selectorExists := secret.Annotations[NamespaceSelectorAnnotation]

	klog.V(3).Infof("manage annotation of %T %s", secret, name)
	var options []client.ListOption
	switch {
	case !allExists && !selectorExists:
		_ = ctx.registry.UnregisterSecret(secret.UID) //NOTE: ignore if secret not already registered
		klog.V(3).Infof("no annotation found on %T %s, ignore synchronization", secret, name)
		return nil
	case allExists && selectorExists:
		_ = ctx.registry.UnregisterSecret(secret.UID) //NOTE: ignore if secret not already registered
		return AnnotationError{fmt.Errorf("annotation '%s' and '%s' cannot be used together", AllNamespacesAnnotation, NamespaceSelectorAnnotation)}
	case allExists:
		if strings.ToLower(onAllNamespaces) != "true" {
			_ = ctx.registry.UnregisterSecret(secret.UID) //NOTE: ignore if secret not already registered
			return AnnotationError{fmt.Errorf("'%s' is not 'true'", AllNamespacesAnnotation)}
		}
	case selectorExists:
		selector, err := labels.Parse(namespaceSelector)
		if err != nil {
			_ = ctx.registry.UnregisterSecret(secret.UID) //NOTE: ignore if secret not already registered
			return AnnotationError{fmt.Errorf("failed to parse '%s': %w", NamespaceSelectorAnnotation, err)}
		}
		options = append(options, client.MatchingLabelsSelector{Selector: selector})
	}

	klog.V(3).Infof("register %T %s internally", secret, name)
	if err := ctx.registry.RegisterSecret(name, secret.UID); err != nil {
		return RegistryError{err}
	}

	copy := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secret.Name,
			Labels:      copyMap(secret.Labels),
			Annotations: copyMap(secret.Annotations),
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
	delete(copy.Annotations, AllNamespacesAnnotation)
	delete(copy.Annotations, NamespaceSelectorAnnotation)

	klog.V(3).Infof("list all namespaces")
	ignoredNamespaces := append(ctx.IgnoredNamespaces, secret.Namespace)
	clusterNamespaces := &corev1.NamespaceList{}
	if err := ctx.client.List(ctx, clusterNamespaces, options...); err != nil {
		return ClientError{fmt.Errorf("failed to list namespaces: %w", err)}
	}

	klog.V(3).Infof("filter namespaces on which secret must be synchronized")
	var namespaces []corev1.Namespace
	for _, namespace := range clusterNamespaces.Items {
		if funk.ContainsString(ignoredNamespaces, namespace.Name) {
			klog.V(4).Infof("namespace %s ignored by '%v'", namespace.Name, ignoredNamespaces)
			continue
		}

		if len(targetNamespaces) > 0 && !funk.ContainsString(targetNamespaces, namespace.Name) {
			klog.V(4).Infof("namespace %s ignored because not targeted", namespace.Name)
			continue
		}

		namespaces = append(namespaces, namespace)
	}

	klog.V(0).Infof("synchronize %T %s on all targeted namespaces (%v) except %v", secret, name, targetNamespaces, ignoredNamespaces)
	for _, namespace := range namespaces {
		klog.V(2).Infof("synchronize %T %s on %s", secret, name, namespace.Name)
		ownedName := types.NamespacedName{Namespace: namespace.Name, Name: secret.Name}
		ownedSecret := &corev1.Secret{}
		copy.Namespace = namespace.Name

		klog.V(3).Infof("fetch %T %s", secret, name)
		err := ctx.client.Get(ctx, ownedName, ownedSecret)
		if errors.IsNotFound(err) {
			klog.V(3).Infof("create %T %s", secret, name)
			if err := ctx.client.Create(ctx, &copy); err != nil {
				return ClientError{fmt.Errorf("failed to create %T %s: %w", ownedSecret, ownedName, err)}
			}
			continue
		} else if err != nil {
			return ClientError{fmt.Errorf("failed to fetch %T %s: %w", ownedSecret, ownedName, err)}
		}

		if !(len(ownedSecret.OwnerReferences) == 1 && ownedSecret.OwnerReferences[0].UID == secret.UID) {
			klog.V(0).Infof("secret %s not owned by %T %s... ignore", ownedName, secret, name)
			continue
		}

		klog.V(3).Infof("update %T %s", secret, name)
		if err = ctx.client.Update(ctx, &copy); err != nil {
			return ClientError{fmt.Errorf("failed to update %T %s: %w", ownedSecret, ownedName, err)}
		}
	}
	return nil
}

func copyMap(o map[string]string) map[string]string {
	c := map[string]string{}

	for k, v := range o {
		c[k] = v
	}
	return c
}
