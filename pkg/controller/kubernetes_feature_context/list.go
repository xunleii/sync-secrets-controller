package kubernetes_feature_context

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// list lists all Kubernetes resource based on the given GroupVersionKind.
func (ctx *KubernetesFeatureContext) list(gvk string, opts ...client.ListOption) ([]*unstructured.Unstructured, error) {
	groupVersionKind, err := GroupVersionKindFrom(gvk)
	if err != nil {
		return nil, err
	}
	// NOTE: can be dangerous but seems working...
	groupVersionKind.Kind += "List"

	kobj, err := scheme.Scheme.New(groupVersionKind)
	if err != nil {
		return nil, err
	}

	err = ctx.List(context.TODO(), kobj, opts...)
	if err != nil {
		return nil, err
	}

	list := unstructured.Unstructured{}
	list.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(kobj)

	var objs []*unstructured.Unstructured
	return objs, list.EachListItem(func(object runtime.Object) error {
		objs = append(objs, object.(*unstructured.Unstructured))
		return nil
	})
}
