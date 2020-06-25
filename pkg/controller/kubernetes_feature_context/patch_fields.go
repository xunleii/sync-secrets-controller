package kubernetes_feature_context

import (
	"fmt"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/types"
)

// LabelizeResource implements the GoDoc step
// - `Kubernetes labelizes <ApiGroupVersionKind> '<NamespacedName>' with '<LabelName>=<LabelValue>'`
// It adds or modifies a specific resource label with the given value.
func LabelizeResource(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes labelizes (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' with '(`+RxFieldPath+`)=(.*)'$`,
		func(groupVersionKind, name, labelName, labelValue string) error {
			patch := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, labelName, labelValue)
			return ctx.patch(groupVersionKind, name, types.MergePatchType, []byte(patch))
		},
	)
}

// RemoveResourceLabel implements the GoDoc step
// - `Kubernetes removes label <LabelName> on <ApiGroupVersionKind> '<NamespacedName>'`
// It removes the given label on the specified resource.
func RemoveResourceLabel(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes removes label '(`+RxFieldPath+`)' on (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)'$`,
		func(label, groupVersionKind, name string) error {
			patch := fmt.Sprintf(`[{"op":"remove","path":"/metadata/labels/%s"}]`, sanitizeJsonPatch(label))
			return ctx.patch(groupVersionKind, name, types.JSONPatchType, []byte(patch))
		},
	)
}

// UpdateResourceLabel implements the GoDoc step
// - `Kubernetes updates label <LabelName> on <ApiGroupVersionKind> '<NamespacedName>' with '<LabelValue>'`
// It updates the given label on the specified resource with the given value.
func UpdateResourceLabel(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes updates label '(`+RxFieldPath+`)' on (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' with '(.*)'$`,
		func(label, groupVersionKind, name, value string) error {
			patch := fmt.Sprintf(`[{"op":"replace","path":"/metadata/labels/%s","value":"%s"}]`, sanitizeJsonPatch(label), value)
			return ctx.patch(groupVersionKind, name, types.JSONPatchType, []byte(patch))
		},
	)
}

// AnnotateResource implements the GoDoc step
// - `Kubernetes annotates <ApiGroupVersionKind> '<NamespacedName>' with '<AnnotationName>=<AnnotationValue>'`
// It adds or modifies a specific resource annotation with the given value.
func AnnotateResource(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes annotates (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' with '(`+RxFieldPath+`)=(.*)'$`,
		func(groupVersionKind, name, annotationName, annotationValue string) error {
			patch := fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`, annotationName, annotationValue)
			return ctx.patch(groupVersionKind, name, types.MergePatchType, []byte(patch))
		},
	)
}

// RemoveResourceAnnotation implements the GoDoc step
// - `Kubernetes removes annotation <AnnotationName> on <ApiGroupVersionKind> '<NamespacedName>'`
// It removes the given annotation on the specified resource.
func RemoveResourceAnnotation(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes removes annotation '(`+RxFieldPath+`)' on (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)'$`,
		func(annotation, groupVersionKind, name string) error {
			patch := fmt.Sprintf(`[{"op":"remove","path":"/metadata/annotations/%s"}]`, sanitizeJsonPatch(annotation))
			return ctx.patch(groupVersionKind, name, types.JSONPatchType, []byte(patch))
		},
	)
}

// UpdateResourceAnnotation implements the GoDoc step
// - `Kubernetes updates annotation <AnnotationName> on <ApiGroupVersionKind> '<NamespacedName>' with '<AnnotationValue>'`
// It updates the given annotation on the specified resource with the given value.
func UpdateResourceAnnotation(ctx *KubernetesFeatureContext, s *godog.Suite) {
	s.Step(
		`^Kubernetes updates annotation '(`+RxFieldPath+`)' on (`+RxGroupVersionKind+`) '(`+RxNamespacedName+`)' with '(.*)'$`,
		func(annotation, groupVersionKind, name, value string) error {
			patch := fmt.Sprintf(`[{"op":"replace","path":"/metadata/annotations/%s","value":"%s"}]`, sanitizeJsonPatch(annotation), value)
			return ctx.patch(groupVersionKind, name, types.JSONPatchType, []byte(patch))
		},
	)
}
