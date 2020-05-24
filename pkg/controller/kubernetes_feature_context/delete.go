package kubernetes_feature_context

import (
	"context"
	"reflect"
	"strings"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

// RemoveResource implements the GoDoc step
// - `Kubernetes removes <ApiGroupVersionKind> '<NamespacedName>'`
// It removes the specified resource.
func RemoveResource(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes removes (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)'$`,
		func(groupVersionKind, name string) error {
			return ctx.delete(groupVersionKind, name)
		},
	)
}

// RemoveMultiResource implements the GoDoc step
// - `Kubernetes removes the following resources <RESOURCES_TABLE>`
// It creates several resources in a row.
func RemoveMultiResource(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes removes the following resources$`,
		func(table ResourceTable) error {
			resources, err := UnmarshalResourceTable(table)
			if err != nil {
				return err
			}

			for _, resource := range resources {
				err := ctx.delete(resource.GroupVersionKind(), resource.NamespacedName())
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
}

// delete deletes a Kubernetes resource based on the given arguments.
func (ctx *KubernetesFeatureContext) delete(gvk, name string) error {
	obj, err := ctx.get(gvk, name)
	if err != nil {
		return err
	}

	err = ctx.deleteWithoutGC(obj)
	if err != nil {
		return err
	}

	if ctx.gc != nil {
		err = ctx.gc(ctx, obj)
	}
	return err
}

// deleteWithoutGC deletes the given Kubernetes resource without calling
// the custom GC.
func (ctx *KubernetesFeatureContext) deleteWithoutGC(obj *unstructured.Unstructured) error {
	// TODO: allow using custom scheme (for Operator)
	kobj, err := scheme.Scheme.New(obj.GroupVersionKind())
	if err != nil {
		return err
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, kobj)
	if err != nil {
		return err
	}

	return ctx.Delete(context.TODO(), kobj)
}

// ManualGC performs a manual and naive garbage collector using the given object as
// owner.
func ManualGC(ctx *KubernetesFeatureContext, delObj *unstructured.Unstructured) error {
	for kind, ktype := range scheme.Scheme.AllKnownTypes() {
		if !strings.HasSuffix(kind.Kind, "List") {
			// ignore non List
			continue
		}
		if kind.Group == "" && strings.HasPrefix(kind.Kind, "API") {
			// ignore API...List
			continue
		}

		kobj := reflect.New(ktype)
		if _, isRuntimeObject := kobj.Interface().(runtime.Object); !isRuntimeObject {
			continue
		}

		err := ctx.List(context.TODO(), kobj.Interface().(runtime.Object))
		if err != nil {
			return err
		}

		{
			// Pre-check if items are available (this check cost less than unmarshalling the entire object)
			items := kobj.Elem().FieldByName("Items")
			if !items.IsValid() || items.IsZero() {
				continue
			}
		}

		obj := unstructured.Unstructured{}
		obj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(kobj.Interface().(runtime.Object))
		if err != nil {
			return err
		}

		items, _, err := unstructured.NestedSlice(obj.Object, "items")
		if err != nil {
			continue
		}

		for _, item := range items {
			if _, isObj := item.(map[string]interface{}); !isObj {
				continue
			}

			obj := unstructured.Unstructured{}
			obj.Object = item.(map[string]interface{})
			if len(obj.GetOwnerReferences()) == 0 {
				continue
			}

			// TODO: how it works on when several owner exists ?
			for _, ownerReference := range obj.GetOwnerReferences() {
				if ownerReference.UID == delObj.GetUID() {
					err := ctx.deleteWithoutGC(&obj)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
