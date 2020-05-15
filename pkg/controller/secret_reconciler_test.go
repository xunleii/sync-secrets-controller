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
	_ = Describe("Reconcile Secret", func() {
		var kube client.Client
		var reg *registrypkg.Registry
		var context *controller.Context
		var reconcilier reconcile.Reconciler

		var uid = uuid.NewUUID()
		var (
			owner = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-secret", Namespace: "default", UID: uid,
					Annotations: map[string]string{},
				},
				Type: "Opaque",
				Data: map[string][]byte{
					"username": []byte(base64.StdEncoding.EncodeToString([]byte("my-app"))),
					"password": []byte(base64.StdEncoding.EncodeToString([]byte("39528$vdg7Jb"))),
				},
			}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: owner.Namespace, Name: owner.Name}}
		)

		BeforeEach(func() {
			kube = fake.NewFakeClientWithScheme(scheme.Scheme)
			reg = registrypkg.New()
			context = controller.NewTestContext(gocontext.TODO(), kube, reg)
			reconcilier = &controller.SecretReconciler{context}

			CreateNamespaces(context, kube)
			Expect(kube.Create(context, owner.DeepCopy())).To(Succeed())
		})

		When("secret doesn't exist", func() {
			owner := owner.DeepCopy()
			It("should ignore the request", func() {
				By("deleting secret", func() {
					Expect(kube.Delete(context, owner)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).ShouldNot(registry.HasSecret(request.NamespacedName, owner.UID))

				secrets := corev1.SecretList{}
				Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
				Expect(secrets.Items).Should(BeEmpty())
			})
		})

		When("secret's namespace is ignored", func() {
			owner := owner.DeepCopy()
			It("should ingore the request", func() {
				context.IgnoredNamespaces = []string{"default"}
				reconcilier = &controller.SecretReconciler{context}

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).ShouldNot(registry.HasSecret(request.NamespacedName, owner.UID))

				secrets := corev1.SecretList{}
				Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
				Expect(secrets).Should(WithoutOwner(owner, BeEmpty()))
			})
		})

		When("no annotation are given", func() {
			owner := owner.DeepCopy()
			It("should ignore the request", func() {
				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).ShouldNot(registry.HasSecret(request.NamespacedName, owner.UID))

				secrets := corev1.SecretList{}
				Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
				Expect(secrets).Should(WithoutOwner(owner, BeEmpty()))
			})
		})

		When("both annotations are given", func() {
			owner := owner.DeepCopy()
			It("should ignore the request", func() {
				By("updating secret with both annotation", func() {
					owner.Annotations = map[string]string{
						controller.AllNamespacesAnnotation:     "true",
						controller.NamespaceSelectorAnnotation: "sync=secret",
					}
					Expect(kube.Update(context, owner)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).ShouldNot(registry.HasSecret(request.NamespacedName, owner.UID))

				secrets := corev1.SecretList{}
				Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
				Expect(secrets).Should(WithoutOwner(owner, BeEmpty()))
			})
		})

		When("'"+controller.AllNamespacesAnnotation+"' annotation is given", func() {
			owner := owner.DeepCopy()
			owner.Annotations = map[string]string{controller.AllNamespacesAnnotation: "true"}

			When("annotation is invalid", func() {
				owner := owner.DeepCopy()
				It("should ignore the request", func() {
					By("updating secret with invalid annotation", func() {
						owner.Annotations = map[string]string{controller.AllNamespacesAnnotation: "invalid"}
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).ShouldNot(registry.HasSecret(request.NamespacedName, owner.UID))

					secrets := corev1.SecretList{}
					Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
					Expect(secrets).Should(WithoutOwner(owner, BeEmpty()))
				})
			})

			When("namespace 'alpha' is ignored", func() {
				It("should create owned secrets on all namespace but 'alpha'", func() {
					context.IgnoredNamespaces = []string{"alpha"}
					reconcilier = &controller.SecretReconciler{context}

					By("updating secret with annotation", func() {
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasSecret(request.NamespacedName, owner.UID))
					Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "kube-system", "kube-public", "kube-lease", "bravo"))

					secrets := corev1.SecretList{}
					Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
					Expect(secrets).Should(HasSecretsOn(owner.Namespace, "kube-system", "kube-public", "kube-lease", "bravo"))
				})
			})

			When("no namespace is ignored", func() {
				It("should create owned secrets on all namespace", func() {
					By("updating secret with annotation", func() {
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasSecret(request.NamespacedName, owner.UID))
					Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "kube-system", "kube-public", "kube-lease", "alpha", "bravo"))

					secrets := corev1.SecretList{}
					Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
					Expect(secrets).Should(HasSecretsOn(owner.Namespace, "kube-system", "kube-public", "kube-lease", "alpha", "bravo"))
				})
			})
		})

		When("'"+controller.NamespaceSelectorAnnotation+"' annotation is given", func() {
			owner := owner.DeepCopy()
			owner.Annotations = map[string]string{controller.NamespaceSelectorAnnotation: "sync=secret"}

			When("annotation is invalid", func() {
				It("should ignore the request", func() {
					owner := owner.DeepCopy()
					By("updating secret with invalid annotation", func() {
						owner.Annotations = map[string]string{controller.NamespaceSelectorAnnotation: "!!!invalid!!!"}
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).ShouldNot(registry.HasSecret(request.NamespacedName, owner.UID))

					secrets := corev1.SecretList{}
					Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
					Expect(secrets).Should(WithoutOwner(owner, BeEmpty()))
				})
			})

			When("namespace 'alpha' is ignored", func() {
				It("should create owned secrets on all matching namespace but 'alpha'", func() {
					context.IgnoredNamespaces = []string{"alpha"}
					reconcilier = &controller.SecretReconciler{context}

					By("updating secret with annotation", func() {
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasSecret(request.NamespacedName, owner.UID))
					Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "bravo"))

					secrets := corev1.SecretList{}
					Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
					Expect(secrets).Should(HasSecretsOn(owner.Namespace, "bravo"))
				})
			})

			When("no namespace is ignored", func() {
				It("should create owned secrets on all matching namespace", func() {
					By("updating secret with annotation", func() {
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasSecret(request.NamespacedName, owner.UID))
					Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "alpha", "bravo"))

					secrets := corev1.SecretList{}
					Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
					Expect(secrets).Should(HasSecretsOn(owner.Namespace, "alpha", "bravo"))
				})
			})
		})

		annotations := map[string]string{controller.AllNamespacesAnnotation: "true"}

		When("secret is created", func() {
			owner := owner.DeepCopy()
			owner.Annotations = annotations

			It("should create all owned secrets and all linked entries in the registry", func() {
				By("updating secret with annotation", func() {
					owner := owner.DeepCopy()
					Expect(kube.Update(context, owner)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).Should(registry.HasSecret(request.NamespacedName, owner.UID))
				Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "kube-system", "kube-public", "kube-lease", "alpha", "bravo"))

				secrets := corev1.SecretList{}
				Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
				Expect(secrets).Should(HasSecretsOn(owner.Namespace, "kube-system", "kube-public", "kube-lease", "alpha", "bravo"))
			})
		})

		When("secret is updated", func() {
			owner := owner.DeepCopy()
			owner.Annotations = annotations

			When("annotation is updated", func() {
				It("should create new owned secret and remove unwanted ones", func() {
					By("updating secret with annotation", func() {
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					By("'reconciliating' updated secret", func() {
						Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
						//NOTE: others fields are test by [When secret is created]
					})

					By("updating secret with another annotation", func() {
						owner := owner.DeepCopy()
						delete(owner.Annotations, controller.AllNamespacesAnnotation)

						owner.Annotations[controller.NamespaceSelectorAnnotation] = "sync=secret"
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasSecret(request.NamespacedName, owner.UID))
					Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "alpha", "bravo"))

					secrets := corev1.SecretList{}
					Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
					Expect(secrets).Should(HasSecretsOn(owner.Namespace, "alpha", "bravo"))
				})
			})

			When("content is updated", func() {
				It("should update owned secret", func() {
					By("updating secret with annotation", func() {
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					By("'reconciliating' updated secret", func() {
						Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
						//NOTE: others fields are test by [When new secret is created]
					})

					By("updating secret with a new data", func() {
						owner.Data["username"] = []byte(base64.StdEncoding.EncodeToString([]byte("my-app2")))
						owner.Data["new_field"] = nil
						Expect(kube.Update(context, owner)).To(Succeed())
					})

					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					Expect(reg).Should(registry.HasSecret(request.NamespacedName, owner.UID))
					Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID, "kube-system", "kube-public", "kube-lease", "alpha", "bravo"))

					secrets := corev1.SecretList{}
					Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
					Expect(secrets).Should(HasSecretsOn(owner.Namespace, "kube-system", "kube-public", "kube-lease", "alpha", "bravo"))

					Expect(secrets).Should(WithTransform(
						TransformSecrets(IgnoreOwner(owner), ExtractData),
						WithTransform(
							func(i interface{}) map[string][]byte {
								data := map[string][]byte{}
								for _, d := range i.([]interface{}) {
									for k, v := range d.(map[string][]byte) {
										data[k] = v
									}
								}
								return data
							},
							Equal(owner.Data),
						),
					))
				})
			})
		})

		When("secret is removed", func() {
			owner := owner.DeepCopy()
			owner.Annotations = annotations

			It("should remove all linked entries in the registry", func() {
				By("updating secret with annotation", func() {
					Expect(kube.Update(context, owner)).To(Succeed())
				})

				By("'reconciliating' updated secret", func() {
					Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
					//NOTE: others fields are test by [When new secret is created]
				})

				By("deleting secret", func() {
					Expect(kube.Delete(context, owner)).To(Succeed())
				})

				Expect(reconcilier.Reconcile(request)).Should(ReconcileSuccessfully())
				Expect(reg).ShouldNot(registry.HasSecret(request.NamespacedName, owner.UID))
				Expect(reg).Should(registry.HasOwnedSecret(request.NamespacedName, owner.UID))

				// NOTE: Owned secrets will be removed by Kubernetes itself, thanks to its GC.
				//       Because the fake-client doesn't implement the latter, we can check if
				//       the reconcilier deletes them or not.
				secrets := corev1.SecretList{}
				Expect(kube.List(context, &secrets, &client.ListOptions{})).To(Succeed())
				Expect(secrets).Should(HasSecretsOn("kube-system", "kube-public", "kube-lease", "alpha", "bravo"))
			})
		})
	})
)
