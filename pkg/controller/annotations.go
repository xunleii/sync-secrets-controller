package controller

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prefixAnnotation            = "secret.sync.klst.pw"
	AllNamespacesAnnotation     = prefixAnnotation + "/all-namespaces"
	NamespaceSelectorAnnotation = prefixAnnotation + "/namespace-selector"
)

// listNamespacesFromAnnotations lists all namespaces based on the secret annotations.
func listNamespacesFromAnnotations(ctx *Context, secret corev1.Secret) ([]string, error) {
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
