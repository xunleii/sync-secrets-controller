package controller_test

import (
	"flag"
	"fmt"
	"io/ioutil"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/xunleii/sync-secrets-controller/pkg/controller"
	registrypkg "github.com/xunleii/sync-secrets-controller/pkg/registry"
)

func init() {
	fset := flag.NewFlagSet("ignore_logs", flag.ExitOnError)
	klog.SetOutput(ioutil.Discard)
	klog.InitFlags(fset)

	_ = fset.Parse([]string{"-logtostderr=false", "-alsologtostderr=false", "-stderrthreshold=10"})
}

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var (
	CreateNamespaces = func(context *controller.Context, kube client.Client) {
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}})).To(Succeed())
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}})).To(Succeed())
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-lease"}})).To(Succeed())
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})).To(Succeed())

		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "alpha", Labels: map[string]string{"sync": "secret"}}})).To(Succeed())
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "bravo", Labels: map[string]string{"sync": "secret"}}})).To(Succeed())
	}
	registry = RegistryMatcher{namespaces: []string{"kube-system", "kube-public", "kube-lease", "default", "alpha", "bravo", "charlie"}}
)

// WithTransform methods
var (
	TransformSecrets = func(transformer ...func(interface{}) interface{}) func(secrets corev1.SecretList) interface{} {
		return func(secrets corev1.SecretList) interface{} {
			var trSecrets []interface{}

		secretIteration:
			for _, original := range secrets.Items {
				var secret interface{} = original
				for _, t := range transformer {
					secret = t(secret)
					if secret == nil {
						continue secretIteration
					}
				}
				trSecrets = append(trSecrets, secret)
			}
			return trSecrets
		}
	}

	TrimSecret = func(i interface{}) interface{} {
		secret := i.(corev1.Secret)
		return corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            secret.Name,
				Namespace:       secret.Namespace,
				UID:             secret.UID,
				Labels:          secret.Labels,
				Annotations:     secret.Annotations,
				OwnerReferences: secret.OwnerReferences,
			},
			Data: secret.Data,
			Type: secret.Type,
		}
	}

	IgnoreOwner = func(owner *corev1.Secret) func(interface{}) interface{} {
		return func(i interface{}) interface{} {
			secret := i.(corev1.Secret)
			if secret.UID == owner.UID {
				return nil
			}
			return secret
		}
	}

	ExtractSecretNamespace = func(i interface{}) interface{} { return i.(corev1.Secret).Namespace }
)

// Custom Gomega matchers
var (
	ReconcileSuccessfully = func() types.GomegaMatcher { return Equal(reconcile.Result{}) }

	WithoutOwner = func(owner *corev1.Secret, matcher types.GomegaMatcher) types.GomegaMatcher {
		return WithTransform(TransformSecrets(IgnoreOwner(owner)), matcher)
	}

	HasSecretsOn = func(namespaces ...string) types.GomegaMatcher {
		inamespaces := make([]interface{}, len(namespaces))
		for i, namespace := range namespaces {
			inamespaces[i] = namespace
		}

		return WithTransform(
			TransformSecrets(ExtractSecretNamespace),
			ConsistOf(inamespaces...),
		)
	}
	ConsistOfSecrets = func(ref corev1.Secret, count int) types.GomegaMatcher {
		getRaw := func(secret corev1.Secret) string {
			secret.SetNamespace("")
			secret.SetGroupVersionKind(schema.GroupVersionKind{})
			secret.SetResourceVersion("")
			return fmt.Sprintf("%#v", secret)
		}

		var expect []interface{}
		refRaw := getRaw(*ref.DeepCopy())
		for i := 0; i < count; i++ {
			expect = append(expect, refRaw)
		}

		return WithTransform(
			func(objs []interface{}) []string {
				return funk.Map(objs, func(i interface{}) string { return getRaw(i.(corev1.Secret)) }).([]string)
			},
			ConsistOf(expect...),
		)
	}
)

type RegistryMatcher struct {
	namespaces []string
}

func (RegistryMatcher) HasSecret(name ktypes.NamespacedName, uid ktypes.UID) types.GomegaMatcher {
	return WithTransform(
		func(r *registrypkg.Registry) *registrypkg.Secret {
			return r.SecretWithUID(uid)
		},
		Equal(&registrypkg.Secret{NamespacedName: name, UID: uid}),
	)
}

func (m RegistryMatcher) HasOwnedSecret(name ktypes.NamespacedName, uid ktypes.UID, onNamespaces ...string) types.GomegaMatcher {
	type NamespacedSecret struct {
		namespace string
		secret    *registrypkg.Secret
	}

	var expectedSecrets []interface{}
	for _, namespace := range onNamespaces {
		expectedSecrets = append(expectedSecrets,
			NamespacedSecret{
				namespace: namespace,
				secret:    &registrypkg.Secret{NamespacedName: name, UID: uid},
			},
		)
	}

	return WithTransform(
		func(r *registrypkg.Registry) []interface{} {
			var availableSecrets []interface{}
			for _, namespace := range m.namespaces {
				if s := r.SecretWithOwnedSecretName(ktypes.NamespacedName{Namespace: namespace, Name: name.Name}); s != nil {
					availableSecrets = append(availableSecrets,
						NamespacedSecret{
							namespace: namespace,
							secret:    &registrypkg.Secret{NamespacedName: name, UID: uid},
						},
					)
				}
			}
			return availableSecrets
		},
		ConsistOf(expectedSecrets...),
	)
}
