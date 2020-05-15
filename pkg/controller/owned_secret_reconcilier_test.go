package controller_test

import (
	gocontext "context"
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/xunleii/sync-secrets-controller/pkg/controller"
	registrypkg "github.com/xunleii/sync-secrets-controller/pkg/registry"
)

var (
	_ = Describe("Reconcile OwnedSecret", func() {
		var kube client.Client
		var reg *registrypkg.Registry
		var context *controller.Context
		var reconcilier reconcile.Reconciler

		var uid = uuid.NewUUID()
		var (
			owner = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-secret", Namespace: "default", UID: uid,
					Annotations: map[string]string{
						controller.NamespaceSelectorAnnotationKey: "sync=secret",
						"unprotected-annotation":                  "true",
						"protected-annotation":                    "true",
					},
					Labels: map[string]string{
						"unprotected-label": "true",
						"protected-label":   "true",
					},
				},
				Type: "Opaque",
				Data: map[string][]byte{
					"username": []byte(base64.StdEncoding.EncodeToString([]byte("my-app"))),
					"password": []byte(base64.StdEncoding.EncodeToString([]byte("39528$vdg7Jb"))),
				},
			}
			owned = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: owner.Name, Namespace: "alpha",
					OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "Secret", Name: owner.Name, UID: owner.UID}},
					Annotations: map[string]string{
						"unprotected-annotation": "true",
					},
					Labels: map[string]string{
						controller.OriginNameLabelsKey:      owner.GetName(),
						controller.OriginNamespaceLabelsKey: owner.GetNamespace(),
						"unprotected-label":                 "true",
					},
				},
				Type: owner.Type,
				Data: owner.Data,
			}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: owned.Namespace, Name: owned.Name}}
		)

		BeforeEach(func() {
			kube = fake.NewFakeClientWithScheme(scheme.Scheme)
			reg = registrypkg.New()
			context = controller.NewTestContext(gocontext.TODO(), kube, reg)
			context.ProtectedAnnotations = []string{"protected-annotation"}
			context.ProtectedLabels = []string{"protected-label"}
			reconcilier = &controller.OwnedSecretReconcilier{context}

			CreateNamespaces(context, kube)
			Expect(kube.Create(context, owner.DeepCopy())).To(Succeed())

			rc := &controller.SecretReconciler{context}
			Expect(rc.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: owner.Namespace, Name: owner.Name}})).Should(ReconcileSuccessfully())
			Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "alpha", "bravo"))
		})

		When("secret owner doesn't exists", func() {
			owner := owner.DeepCopy()
			It("should ignore the reconciliation", func() {
				By("deleting owner secret", func() {
					Expect(kube.Delete(context, owner)).To(Succeed())
					Expect(reg.UnregisterSecret(owner.UID)).To(Succeed())
					Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID))

					rc := &controller.SecretReconciler{context}
					Expect(rc.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: owner.Namespace, Name: owner.Name}})).Should(ReconcileSuccessfully())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID))
			})
		})

		When("owned secret is created", func() {
			It("should do nothing", func() {
				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "alpha", "bravo"))
			})
		})

		When("owned secret is updated", func() {
			owned := owned.DeepCopy()
			It("should restore the owner secret state", func() {
				By("updating the owned secret", func() {
					owned := owned.DeepCopy()
					owned.Labels = map[string]string{"label": "value"}
					Expect(kube.Update(context, owned)).To(Succeed())

					secret := corev1.Secret{}
					Expect(kube.Get(context, request.NamespacedName, &secret)).To(Succeed())
					Expect(secret).ShouldNot(WithTransform(TrimSecret, Equal(owned)))
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())

				secret := corev1.Secret{}
				Expect(kube.Get(context, request.NamespacedName, &secret)).To(Succeed())
				Expect(secret).Should(WithTransform(TrimSecret, Equal(*owned)))
			})
		})

		When("owned secret is removed", func() {
			owned := owned.DeepCopy()
			It("should restore the owner secret state", func() {
				By("deleting owned secret", func() {
					Expect(kube.Delete(context, owned)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())

				secret := corev1.Secret{}
				Expect(kube.Get(context, request.NamespacedName, &secret)).To(Succeed())
				Expect(secret).Should(WithTransform(TrimSecret, Equal(*owned)))
			})
		})
	})
)
