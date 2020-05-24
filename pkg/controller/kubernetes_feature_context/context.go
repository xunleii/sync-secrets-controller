package kubernetes_feature_context

import (
	"github.com/cucumber/godog"
	"github.com/cucumber/messages-go/v10"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	RxDNSChar          = `[a-z0-9\-.]`
	RxGroupVersionKind = `[\w/]+`
	RxNamespacedName   = RxDNSChar + `+(?:/` + RxDNSChar + `+)?`
	RxFieldPath        = `[^=:]+?`
)

type KubernetesFeatureContextOption func(ctx *KubernetesFeatureContext) error
type KubernetesFeatureContext struct {
	client.Client
	gc func(*KubernetesFeatureContext, *unstructured.Unstructured) error
}

func FakeClient(ctx *KubernetesFeatureContext) error {
	ctx.Client = fake.NewFakeClientWithScheme(scheme.Scheme)
	return nil
}

func UseCustomGC(gc func(*KubernetesFeatureContext, *unstructured.Unstructured) error) func(ctx *KubernetesFeatureContext) error {
	return func(ctx *KubernetesFeatureContext) error {
		ctx.gc = gc
		return nil
	}
}

func FeatureContext(s *godog.Suite, opts ...KubernetesFeatureContextOption) (*KubernetesFeatureContext, error) {
	ctx := &KubernetesFeatureContext{}

	s.BeforeScenario(func(pickle *messages.Pickle) {
		//TODO: check errors
		for _, opt := range opts {
			_ = opt(ctx)
		}
	})

	// Create resources
	CreateSingleResource(ctx, s)
	CreateSingleResourceWith(ctx, s)
	CreateSingleResourceFrom(ctx, s)
	CreateMultiResources(ctx, s)

	// Update resources

	// Delete resources
	RemoveResource(ctx, s)
	RemoveMultiResource(ctx, s)

	// Get resources
	ResourceExists(ctx, s)
	ResourceNotExists(ctx, s)
	ResourceIsSimilarTo(ctx, s)
	ResourceIsNotSimilarTo(ctx, s)
	ResourceIsEqualTo(ctx, s)
	ResourceIsNotEqualTo(ctx, s)

	ResourceHasField(ctx, s)
	ResourceDoesntHaveField(ctx, s)
	ResourceHasFieldEqual(ctx, s)
	ResourceHasFieldNotEqual(ctx, s)
	ResourceHasLabel(ctx, s)
	ResourceDoesntHaveLabel(ctx, s)
	ResourceHasLabelEqual(ctx, s)
	ResourceHasLabelNotEqual(ctx, s)
	ResourceHasAnnotation(ctx, s)
	ResourceDoesntHaveAnnotation(ctx, s)
	ResourceHasAnnotationEqual(ctx, s)
	ResourceHasAnnotationNotEqual(ctx, s)

	// List resources

	// Patch resources
	PatchResourceWith(ctx, s)
	LabelizeResource(ctx, s)
	UpdateResourceLabel(ctx, s)
	RemoveResourceLabel(ctx, s)
	AnnotateResource(ctx, s)
	UpdateResourceAnnotation(ctx, s)
	RemoveResourceAnnotation(ctx, s)

	return ctx, nil
}
