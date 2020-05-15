package controller

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// syncing type annotations
	NamespaceAllAnnotationKey      = "secret.sync.klst.pw/all-namespaces"
	NamespaceSelectorAnnotationKey = "secret.sync.klst.pw/namespace-selector"

	// origin annotations (based on the idea of github.com/appscode/kubed)
	OriginNameLabelsKey      = "secret.sync.klst.pw/origin.name"
	OriginNamespaceLabelsKey = "secret.sync.klst.pw/origin.namespace"
)

// listNamespacesFromAnnotations lists all namespaces based on the secret annotations.
func listNamespacesFromAnnotations(ctx *Context, secret corev1.Secret) ([]string, error) {
	var options []client.ListOption

	allNamespaces, hasAllNamespace := secret.Annotations[NamespaceAllAnnotationKey]
	namespaceSelector, hasNamespaceSelector := secret.Annotations[NamespaceSelectorAnnotationKey]

	var err error
	switch {
	case hasAllNamespace && hasNamespaceSelector:
		err = AnnotationError{fmt.Errorf("annotation '%s' and '%s' cannot be used together", NamespaceAllAnnotationKey, NamespaceSelectorAnnotationKey)}
	case hasAllNamespace:
		if strings.ToLower(allNamespaces) != "true" {
			err = AnnotationError{fmt.Errorf("'%s' is not 'true'", NamespaceAllAnnotationKey)}
		}
	case hasNamespaceSelector:
		var selector labels.Selector
		selector, err = labels.Parse(namespaceSelector)
		if err != nil {
			err = AnnotationError{fmt.Errorf("failed to parse '%s': %w", NamespaceSelectorAnnotationKey, err)}
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

// assignOriginMetadata assign to the secret some metadata that come from the
// original secret.
func assignOriginMetadata(secret, origin *corev1.Secret) *corev1.Secret {
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}

	secret.Labels[OriginNameLabelsKey] = origin.Name
	secret.Labels[OriginNamespaceLabelsKey] = origin.Namespace
	return secret
}

// excludeProtectedMetadata removes all protected labels or annotations from the
// given secret. A protected labels (or annotations) is a labels which must not
// be copied to an owned secret. Theses protected fields are provided by the
// end user.
func excludeProtectedMetadata(ctx *Context, secret *corev1.Secret) *corev1.Secret {
	delete(secret.Annotations, NamespaceAllAnnotationKey)
	delete(secret.Annotations, NamespaceSelectorAnnotationKey)
	for _, annotation := range ctx.ProtectedAnnotations {
		delete(secret.Annotations, annotation)
	}
	for _, label := range ctx.ProtectedLabels {
		delete(secret.Labels, label)
	}
	return secret
}
