package kubernetes_feature_context

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

// ResourceExists implements the GoDoc step
// - `Kubernetes has <ApiGroupVersionKind> '<NamespacedName>'`
// It validates the fact that Kubernetes has the specified resource.
func ResourceExists(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes has (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)'$`,
		func(groupVersionKind, name string) error {
			_, err := ctx.get(groupVersionKind, name)
			return err
		},
	)
}

// ResourceNotExists implements the GoDoc step
// - `Kubernetes doesn't have <ApiGroupVersionKind> '<NamespacedName>'`
// It validates the fact that Kubernetes doesn't have the specified resource.
func ResourceNotExists(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes doesn't have (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)'$`,
		func(groupVersionKind, name string) error {
			_, err := ctx.get(groupVersionKind, name)
			switch {
			case errors.IsNotFound(err):
				return nil
			case err != nil:
				return err
			default:
				return fmt.Errorf(`%s "%s" found`, groupVersionKind, name)
			}
		},
	)
}

// ResourceIsSimilarTo implements the GoDoc step
// - `Kubernetes resource <ApiGroupVersionKind> '<NamespacedName>' is similar to '<NamespacedName>'`
// It compares two resources in order to determine if they are similar.
//
// NOTE: Two resources are similar if all fields except 'medatata' are the same.
func ResourceIsSimilarTo(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes resource (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' is similar to '(`+RxNamespacedName+`)'$`,
		func(groupVersionKind, lname, rname string) (err error) {
			lobj, err := ctx.get(groupVersionKind, lname)
			if err != nil {
				return err
			}
			delete(lobj.Object, "metadata")

			robj, err := ctx.get(groupVersionKind, rname)
			if err != nil {
				return err
			}
			delete(robj.Object, "metadata")

			diff, err := diffObjects(lobj, robj)
			switch {
			case err != nil:
				return err
			case diff != "":
				return fmt.Errorf("resources %s '%s' and '%s' are not similar: %s", groupVersionKind, lname, rname, diff)
			}
			return nil
		},
	)
}

// ResourceIsNotSimilarTo implements the GoDoc step
// - `Kubernetes resource <ApiGroupVersionKind> '<NamespacedName>' is not similar to '<NamespacedName>'`
// It compares two resources in order to determine if they are not similar.
//
// NOTE: Two resources are similar if all fields except 'medatata' are the same.
func ResourceIsNotSimilarTo(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes resource (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' is not similar to '(`+RxNamespacedName+`)'$`,
		func(groupVersionKind, lname, rname string) (err error) {
			lobj, err := ctx.get(groupVersionKind, lname)
			if err != nil {
				return err
			}
			delete(lobj.Object, "metadata")

			robj, err := ctx.get(groupVersionKind, rname)
			if err != nil {
				return err
			}
			delete(robj.Object, "metadata")

			diff := gojsondiff.New().CompareObjects(lobj.Object, robj.Object)
			if !diff.Modified() {
				return fmt.Errorf("resources %s '%s' and '%s' are similar", groupVersionKind, lname, rname)
			}
			return nil
		},
	)
}

// ResourceIsEqualTo implements the GoDoc step
// - `Kubernetes resource <ApiGroupVersionKind> '<NamespacedName>' is equal to '<NamespacedName>'`
// It compares two resources in order to determine if they are equal.
//
// NOTE: Two resources are equal if all fields except unique fields ('metadata.name',
//       'metadata.namespace', 'metadata.uid' and 'metadata.resourceVersion') are the same.
func ResourceIsEqualTo(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes resource (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' is equal to '(`+RxNamespacedName+`)'$`,
		func(groupVersionKind, lname, rname string) (err error) {
			lobj, err := ctx.get(groupVersionKind, lname)
			if err != nil {
				return err
			}
			lobj.SetName("")
			lobj.SetNamespace("")
			lobj.SetUID("")
			lobj.SetResourceVersion("")

			robj, err := ctx.get(groupVersionKind, rname)
			if err != nil {
				return err
			}
			robj.SetName("")
			robj.SetNamespace("")
			robj.SetUID("")
			robj.SetResourceVersion("")

			diff, err := diffObjects(lobj, robj)
			switch {
			case err != nil:
				return err
			case diff != "":
				return fmt.Errorf("resources %s '%s' and '%s' are not equal: %s", groupVersionKind, lname, rname, diff)
			}
			return nil
		},
	)
}

// ResourceIsNotEqualTo implements the GoDoc step
// - `Kubernetes resource <ApiGroupVersionKind> '<NamespacedName>' is not equal to '<NamespacedName>'`
// It compares two resources in order to determine if they are not equal.
//
// NOTE: Two resources are equal if all fields except unique fields ('metadata.name',
//       'metadata.namespace', 'metadata.uid' and 'metadata.resourceVersion') are the same.
func ResourceIsNotEqualTo(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes resource (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' is not equal to '(`+RxNamespacedName+`)'$`,
		func(groupVersionKind, lname, rname string) (err error) {
			lobj, err := ctx.get(groupVersionKind, lname)
			if err != nil {
				return err
			}
			lobj.SetName("")
			lobj.SetNamespace("")
			lobj.SetUID("")
			lobj.SetResourceVersion("")

			robj, err := ctx.get(groupVersionKind, rname)
			if err != nil {
				return err
			}
			robj.SetName("")
			robj.SetNamespace("")
			robj.SetUID("")
			robj.SetResourceVersion("")

			diff := gojsondiff.New().CompareObjects(lobj.Object, robj.Object)
			if !diff.Modified() {
				return fmt.Errorf("resources %s '%s' and '%s' are equal", groupVersionKind, lname, rname)
			}
			return nil
		},
	)
}

// get fetches a Kubernetes resource based on the give arguments.
func (ctx *KubernetesFeatureContext) get(gvk, name string) (*unstructured.Unstructured, error) {
	groupVersionKind, err := GroupVersionKindFrom(gvk)
	if err != nil {
		return nil, err
	}
	namespacedName, err := NamespacedNameFrom(name)
	if err != nil {
		return nil, err
	}

	// TODO: allow using custom scheme (for Operator)
	kobj, err := scheme.Scheme.New(groupVersionKind)
	if err != nil {
		return nil, err
	}

	err = ctx.Get(context.TODO(), namespacedName, kobj)
	if err != nil {
		return nil, err
	}

	obj := unstructured.Unstructured{}
	obj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(kobj)
	return &obj, err
}

// diffObjects return a readable diff if the given objects are different.
func diffObjects(lobj, robj *unstructured.Unstructured) (string, error) {
	diff := gojsondiff.New().CompareObjects(lobj.Object, robj.Object)
	if diff.Modified() {
		outDiff, err := formatter.
			NewAsciiFormatter(lobj.Object, formatter.AsciiFormatterConfig{Coloring: false, ShowArrayIndex: true}).
			Format(diff)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("\n%s", outDiff), nil
	}
	return "", nil
}
