package kubernetes_feature_context

import (
	"context"

	"github.com/cucumber/godog"
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

// delete deletes a Kubernetes resource based on the give arguments.
func (ctx *KubernetesFeatureContext) delete(gvk, name string) error {
	obj, err := ctx.get(gvk, name)
	if err != nil {
		return err
	}

	kobj, err := scheme.Scheme.New(obj.GroupVersionKind())
	if err != nil {
		return err
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, kobj)
	if err != nil {
		return err
	}

	// TODO: implement GC for fake client
	return ctx.Delete(context.TODO(), kobj)
}
