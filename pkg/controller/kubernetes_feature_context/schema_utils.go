package kubernetes_feature_context

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// GroupVersionKindFrom converts a string to a GroupVersionKind.
func GroupVersionKindFrom(gvk string) (schema.GroupVersionKind, error) {
	items := strings.Split(gvk, "/")

	switch len(items) {
	case 3:
		return schema.GroupVersionKind{Group: items[0], Version: items[1], Kind: items[2]}, nil
	case 2:
		return schema.GroupVersionKind{Version: items[0], Kind: items[1]}, nil
	default:
		return schema.GroupVersionKind{}, fmt.Errorf("invalid GroupVersionKind '%s'", gvk)
	}
}

// NamespacedNameFrom converts a string to a NamespacedName.
func NamespacedNameFrom(name string) (types.NamespacedName, error) {
	items := strings.Split(name, "/")

	switch len(items) {
	case 2:
		return types.NamespacedName{Namespace: items[0], Name: items[1]}, nil
	case 1:
		return types.NamespacedName{Name: items[0]}, nil
	default:
		return types.NamespacedName{}, fmt.Errorf("invalid NamespacedName '%s'", name)
	}
}
