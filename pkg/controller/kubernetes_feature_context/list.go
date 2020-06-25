package kubernetes_feature_context

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/thoas/go-funk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CountResources implements the GoDoc step
// - `Kubernetes has <NumberResources> <ApiGroupVersionKind>`
// It compare the current number of a specific resource with the given number.
func CountResources(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes has (\d+) (`+RxGroupVersionKind+`)$`,
		func(n int, groupVersionKind string) error {
			objs, err := ctx.list(groupVersionKind)
			if err != nil {
				return err
			}

			if len(objs) != n {
				if len(objs) == 0 {
					return fmt.Errorf("no %s found", groupVersionKind)
				}

				items := funk.Map(objs, func(obj *unstructured.Unstructured) string {
					if obj.GetNamespace() == "" {
						return obj.GetName()
					}
					return obj.GetNamespace() + "/" + obj.GetName()
				})
				return fmt.Errorf("%d %s found (%s)", len(objs), groupVersionKind, strings.Join(items.([]string), ","))
			}
			return nil
		},
	)
}

// CountNamespacedResources implements the GoDoc step
// - `Kubernetes has <NumberResources> <ApiGroupVersionKind> in namespace '<Namespace>'`
// It compare the current number of a specific resource with the given number.
func CountNamespacedResources(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes has (\d+) (`+RxGroupVersionKind+`) in namespace '(`+RxDNSChar+`+)'$`,
		func(n int, groupVersionKind, namespace string) error {
			objs, err := ctx.list(groupVersionKind, &client.ListOptions{Namespace: namespace})
			if err != nil {
				return err
			}

			if len(objs) != n {
				if len(objs) == 0 {
					return fmt.Errorf("no %s found in namespace '%s'", groupVersionKind, namespace)
				}

				items := funk.Map(objs, func(obj *unstructured.Unstructured) string {
					if obj.GetNamespace() == "" {
						return obj.GetName()
					}
					return obj.GetNamespace() + "/" + obj.GetName()
				})
				return fmt.Errorf("%d %s found in namespace '%s' (%s)", len(objs), groupVersionKind, namespace, strings.Join(items.([]string), ","))
			}
			return nil
		},
	)
}

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
	if err != nil {
		return nil, err
	}

	var objs []*unstructured.Unstructured
	return objs, list.EachListItem(func(object runtime.Object) error {
		objs = append(objs, object.(*unstructured.Unstructured))
		return nil
	})
}
