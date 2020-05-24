package kubernetes_feature_context

import (
	"context"
	"io/ioutil"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

// CreateSingleResource implements the GoDoc step
// - `Kubernetes must have <ApiGroupVersionKind> '<NamespacedName>'`
// - `Kubernetes creates a new <ApiGroupVersionKind> '<NamespacedName>'`
// It creates a new resource, without any specific fields.
func CreateSingleResource(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes (?:must have|creates a new) (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)'$`,
		func(groupVersionKind, resourceName string) error {
			return ctx.create(groupVersionKind, resourceName, unstructured.Unstructured{})
		},
	)
}

// CreateSingleResourceWith implements the GoDoc step
// - `Kubernetes must have <ApiGroupVersionKind> '<NamespacedName>' with <YAML>`
// - `Kubernetes creates a new <ApiGroupVersionKind> '<NamespacedName>' with <YAML>`
// It creates a new resource, with the given definition.
func CreateSingleResourceWith(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes (?:must have|creates a new) (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' with$`,
		func(groupVersionKind, resourceName string, yamlObj YamlDocString) error {
			obj, err := UnmarshalYamlDocString(yamlObj)
			if err != nil {
				return err
			}

			return ctx.create(groupVersionKind, resourceName, unstructured.Unstructured{Object: obj})
		},
	)
}

// CreateSingleResourceFrom implements the GoDoc step
// - `Kubernetes must have <ApiGroupVersionKind> '<NamespacedName>' from <filename>`
// - `Kubernetes creates a new <ApiGroupVersionKind> '<NamespacedName>' from <filename>`
// It creates a new resource, with then definition available in the given filename.
func CreateSingleResourceFrom(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes (?:must have|creates a new) (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' from (.+)$`,
		func(groupVersionKind, resourceName, fileName string) error {
			data, err := ioutil.ReadFile(fileName)
			if err != nil {
				return err
			}

			var obj unstructured.Unstructured
			err = yaml.Unmarshal(data, &obj.Object)
			if err != nil {
				return err
			}

			return ctx.create(groupVersionKind, resourceName, obj)
		},
	)
}

// CreateMultiResources implements the GoDoc step
// - `Kubernetes must have the following resources <RESOURCES_TABLE>`
// - `Kubernetes creates the following resources <RESOURCES_TABLE>`
// It creates several resources in a row, without any specific fields (useful for Namespaces).
func CreateMultiResources(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes (?:must have|creates) the following resources$`,
		func(table ResourceTable) error {
			resources, err := UnmarshalResourceTable(table)
			if err != nil {
				return err
			}

			for _, resource := range resources {
				err := ctx.create(resource.GroupVersionKind(), resource.NamespacedName(), unstructured.Unstructured{})
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
}

// create creates a Kubernetes resource based on the give arguments.
func (ctx *KubernetesFeatureContext) create(gvk, name string, obj unstructured.Unstructured) error {
	groupVersionKind, err := GroupVersionKindFrom(gvk)
	if err != nil {
		return err
	}
	namespacedName, err := NamespacedNameFrom(name)
	if err != nil {
		return err
	}

	obj.SetGroupVersionKind(groupVersionKind)
	obj.SetUID(types.UID(uuid.New().String()))
	obj.SetName(namespacedName.Name)
	obj.SetNamespace(namespacedName.Namespace)

	// TODO: allow using custom scheme (for Operator)
	kobj, err := scheme.Scheme.New(groupVersionKind)
	if err != nil {
		return err
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, kobj)
	if err != nil {
		return err
	}
	return ctx.Create(context.TODO(), kobj)
}
