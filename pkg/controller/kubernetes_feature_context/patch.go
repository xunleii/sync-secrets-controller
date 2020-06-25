package kubernetes_feature_context

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PatchResourceWith implements the GoDoc step
// - `Kubernetes patches <ApiGroupVersionKind> '<NamespacedName>' with <YAML>`
// It patches a specific resource with the given patch (it use StrategicMergePatchType...
// see https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md
// for more information).
func PatchResourceWith(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes patches (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' with$`,
		func(groupVersionKind, name string, content pickleDocString) error {
			patch, err := yamlToJson(content.Content)
			if err != nil {
				return err
			}
			return ctx.patch(groupVersionKind, name, types.StrategicMergePatchType, patch)
		},
	)
}

// patch patches a Kubernetes resource based on the give arguments.
func (ctx *KubernetesFeatureContext) patch(gvk, name string, pt types.PatchType, data []byte) error {
	groupVersionKind, err := GroupVersionKindFrom(gvk)
	if err != nil {
		return err
	}
	namespacedName, err := NamespacedNameFrom(name)
	if err != nil {
		return err
	}

	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(groupVersionKind)
	err = ctx.Get(context.TODO(), namespacedName, &obj)
	if err != nil {
		return err
	}

	return ctx.Patch(context.TODO(), &obj, client.RawPatch(pt, data))
}

// yamlToJson converts naively YAML string to JSON []byte.
func yamlToJson(in string) ([]byte, error) {
	var x map[string]interface{}
	if err := yaml.Unmarshal([]byte(in), &x); err != nil {
		return nil, err
	}
	return json.Marshal(x)
}

// sanitizeJsonPatch replace all '/' and '~' in the given JsonPath expression.
func sanitizeJsonPatch(expr string) string {
	return strings.ReplaceAll(strings.ReplaceAll(expr, "~", "~0"), "/", "~1")
}
