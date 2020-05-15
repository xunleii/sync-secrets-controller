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
	_ = Describe("Reconcile Namespace", func() {
		var kube client.Client
		var reg *registrypkg.Registry
		var context *controller.Context
		var reconcilier reconcile.Reconciler

		var (
			secretA = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-secret-a", Namespace: "default", UID: uuid.NewUUID(),
					Annotations: map[string]string{
						controller.NamespaceSelectorAnnotation: "sync=secret",
					},
				},
				Type: "Opaque",
				Data: map[string][]byte{
					"username": []byte(base64.StdEncoding.EncodeToString([]byte("my-app"))),
					"password": []byte(base64.StdEncoding.EncodeToString([]byte("39528$vdg7Jb"))),
				},
			}
			secretB = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-secret-b", Namespace: "default", UID: uuid.NewUUID(),
					Annotations: map[string]string{
						controller.AllNamespacesAnnotation: "true",
					},
				},
				Type: "Opaque",
				Data: map[string][]byte{
					"username": []byte(base64.StdEncoding.EncodeToString([]byte("my-app"))),
					"password": []byte(base64.StdEncoding.EncodeToString([]byte("39528$vdg7Jb"))),
				},
			}
		)

		BeforeEach(func() {
			kube = fake.NewFakeClientWithScheme(scheme.Scheme)
			reg = registrypkg.New()
			context = controller.NewTestContext(gocontext.TODO(), kube, reg)
			reconcilier = &controller.NamespaceReconciler{context}

			CreateNamespaces(context, kube)
			Expect(kube.Create(context, secretA.DeepCopy())).To(Succeed())
			Expect(kube.Create(context, secretB.DeepCopy())).To(Succeed())

			rc := &controller.SecretReconciler{context}

			request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}}
			Expect(rc.Reconcile(request)).Should(ReconcileSuccessfully())
			Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, secretA.UID, "alpha", "bravo"))

			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: secretB.Namespace, Name: secretB.Name}}
			Expect(rc.Reconcile(request)).Should(ReconcileSuccessfully())
			Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, secretB.UID, "kube-system", "kube-public", "kube-lease", "alpha", "bravo"))
		})

		When("namespace label doesn't match annotations", func() {
			It("should ignore the request", func() {
				By("creating new namespace 'charlie' without labels", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie"}}
					Expect(kube.Create(context, &namespace)).To(Succeed())
				})

				request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "charlie"}}
				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo"))
				Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretB.Namespace, Name: secretB.Name}, secretB.UID, "kube-system", "kube-public", "kube-lease", "alpha", "bravo", "charlie"))
			})
		})

		When("namespace label matches annotations", func() {
			When("namespace match only '"+controller.AllNamespacesAnnotation+"'", func() {
				It("should synchronize only secrets with '"+controller.AllNamespacesAnnotation+"'", func() {
					By("creating new namespace 'charlie' without labels", func() {
						namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie"}}
						Expect(kube.Create(context, &namespace)).To(Succeed())
					})

					request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "charlie"}}
					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo"))
					Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretB.Namespace, Name: secretB.Name}, secretB.UID, "kube-system", "kube-public", "kube-lease", "alpha", "bravo", "charlie"))
				})
			})

			When("namespace match all annotations", func() {
				It("should synchronize all secrets", func() {
					By("creating new namespace 'charlie' with label 'sync=secret", func() {
						namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie", Labels: map[string]string{"sync": "secret"}}}
						Expect(kube.Create(context, &namespace)).To(Succeed())
					})

					request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "charlie"}}
					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo", "charlie"))
					Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretB.Namespace, Name: secretB.Name}, secretB.UID, "kube-system", "kube-public", "kube-lease", "alpha", "bravo", "charlie"))
				})
			})
		})

		When("namespace is updated with a matching label", func() {
			request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "charlie"}}
			It("should synchronize all secrets", func() {

				By("creating new namespace 'charlie' without labels", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie"}}
					Expect(kube.Create(context, &namespace)).To(Succeed())
					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo"))
				})

				By("updating 'charlie' namespace with label 'sync=secret'", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie", Labels: map[string]string{"sync": "secret"}}}
					Expect(kube.Update(context, &namespace)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo", "charlie"))
			})
		})

		When("namespace is updated by removing matching label", func() {
			It("should desynchronize secrets", func() {
				request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "charlie"}}

				By("creating new namespace 'charlie' with label 'sync=secret'", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie", Labels: map[string]string{"sync": "secret"}}}
					Expect(kube.Create(context, &namespace)).To(Succeed())
					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo", "charlie"))
				})

				By("removing label on 'charlie' namespace", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie"}}
					Expect(kube.Update(context, &namespace)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo"))
			})
		})

		When("namespace is updated by editing matching annotations", func() {
			It("should synchronize all secrets", func() {
				request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "charlie"}}

				By("creating new namespace 'charlie' with label 'sync=secret'", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie", Labels: map[string]string{"sync": "secret"}}}
					Expect(kube.Create(context, &namespace)).To(Succeed())
					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo", "charlie"))
				})

				By("updating label on 'charlie' namespace", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie", Labels: map[string]string{"sync": "no-secret"}}}
					Expect(kube.Update(context, &namespace)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo"))

				By("updating label on 'charlie' namespace", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie", Labels: map[string]string{"sync": "secret"}}}
					Expect(kube.Update(context, &namespace)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo", "charlie"))
			})
		})

		When("namespace is removed", func() {
			It("should desynchronize all secrets", func() {
				request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "charlie"}}

				By("creating new namespace 'charlie' with label 'sync=secret'", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie", Labels: map[string]string{"sync": "secret"}}}
					Expect(kube.Create(context, &namespace)).To(Succeed())
					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo", "charlie"))
				})

				By("removing 'charlie' namespace", func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie", Labels: map[string]string{"sync": "secret"}}}
					Expect(kube.Delete(context, &namespace)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasOwnedSecret(types.NamespacedName{Namespace: secretA.Namespace, Name: secretA.Name}, secretA.UID, "alpha", "bravo"))
			})
		})
	})
)
