package controller

import (
	gocontext "context"
	"encoding/base64"
	"sync"

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
)

var (
	setupSpecs = func(client *client.Client, controller *reconcileSecret) (*corev1.Secret, []metav1.OwnerReference) {
		var (
			owner = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "shared-secret", Namespace: "default", UID: uuid.NewUUID()},
				Data:       map[string][]byte{"owner": []byte(base64.StdEncoding.EncodeToString([]byte("the-owner-value")))},
			}
			ownerReferences = []metav1.OwnerReference{
				{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Secret", Name: owner.Name, UID: owner.UID},
			}
		)

		*client = fake.NewFakeClientWithScheme(scheme.Scheme)
		*controller = reconcileSecret{context: &context{owners: &sync.Map{}}, client: *client}

		It("must create default namespaces", func() {
			namespaces := []string{"kube-system", "kube-public", "default"}
			for _, namespace := range namespaces {
				Expect((*client).Create(gocontext.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).
					To(Succeed())
			}
		})

		It("must create original secret", func() {
			Expect((*client).Create(gocontext.TODO(), owner)).To(Succeed())
		})

		return owner, ownerReferences
	}
	reconcileUpdatedSecret = func(client client.Client, controller *reconcileSecret, owner *corev1.Secret) {
		ownerRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: owner.Name, Namespace: owner.Namespace}}

		Expect(client.Update(gocontext.TODO(), owner)).To(Succeed())
		res, err := controller.Reconcile(ownerRequest)
		Expect(res).Should(Equal(reconcile.Result{}))
		Expect(err).Should(Succeed())
	}

	_ = Describe("Reconcile secrets without annotation", func() {
		var client client.Client
		var controller reconcileSecret

		owner, _ := setupSpecs(&client, &controller)

		It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
		It("should contain only one secret", func() {
			secrets := &corev1.SecretList{}
			Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
			Expect(secrets.Items).Should(HaveLen(1))
		})
	})
	_ = Describe("Reconcile secrets with both annotations", func() {
		var client client.Client
		var controller reconcileSecret

		owner, _ := setupSpecs(&client, &controller)

		owner.Annotations = map[string]string{
			allNamespacesAnnotation:     "true",
			namespaceSelectorAnnotation: "need-shared-secret=true",
		}
		It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
		It("should contain only one secret", func() {
			secrets := &corev1.SecretList{}
			Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
			Expect(secrets.Items).Should(HaveLen(1))
		})
	})
	_ = Describe("Reconcile secrets with '"+allNamespacesAnnotation+"' annotation", func() {
		var client client.Client
		var controller reconcileSecret

		owner, ownerReferences := setupSpecs(&client, &controller)

		When("when is invalid", func() {
			owner := owner.DeepCopy()
			owner.Annotations = map[string]string{allNamespacesAnnotation: "false"}

			It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
			It("should contain one secret & zero replicated secret", func() {
				secrets := &corev1.SecretList{}
				Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
				Expect(secrets.Items).Should(HaveLen(1))
				Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), HaveLen(0)))
			})
		})

		When("is 'true'", func() {
			owner := owner.DeepCopy()
			owner.Annotations = map[string]string{allNamespacesAnnotation: "true"}

			It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
			It("should contain three secrets & two replicated secrets", func() {
				secrets := &corev1.SecretList{}
				Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
				Expect(secrets.Items).Should(HaveLen(3))
				Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), And(
					WithTransform(GetNames, ConsistOf(owner.Name, owner.Name)),
					WithTransform(GetNamespaces, ConsistOf("kube-system", "kube-public")),
					WithTransform(GetOwnerReferences, ConsistOf(ownerReferences, ownerReferences)),
				)))
			})

			When("annotations are updated", func() {
				owner.Annotations["foo"] = "bar"

				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
				It("should contain two replicated secrets with the new annotation", func() {
					expectedAnnotations := map[string]string{"foo": "bar"}
					secrets := &corev1.SecretList{}

					Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(3))
					Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), And(
						WithTransform(GetNames, ConsistOf(owner.Name, owner.Name)),
						WithTransform(GetNamespaces, ConsistOf("kube-system", "kube-public")),
						WithTransform(GetAnnotations, ConsistOf(expectedAnnotations, expectedAnnotations)),
					)))
				})
			})

			When("labels are updated", func() {
				owner.Labels = map[string]string{"foo": "bar"}

				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
				It("should contain two replicated secrets with the new annotation", func() {
					expectedLabels := map[string]string{"foo": "bar"}
					secrets := &corev1.SecretList{}

					Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(3))
					Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), And(
						WithTransform(GetNames, ConsistOf(owner.Name, owner.Name)),
						WithTransform(GetNamespaces, ConsistOf("kube-system", "kube-public")),
						WithTransform(GetLabels, ConsistOf(expectedLabels, expectedLabels)),
					)))
				})
			})

			Context("with 'kube-system' namespace ignored", func() {
				controller := controller.DeepCopy()
				controller.ignoredNamespaces = []string{"kube-system"}

				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, controller, owner) })
				It("should contain two secrets & one replicated secrets", func() {
					//TODO: removing mechanism is not implemented
					Skip("removing mechanism is not implemented")

					secrets := &corev1.SecretList{}
					Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(2))
					Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), And(
						WithTransform(GetNames, ConsistOf(owner.Name)),
						WithTransform(GetNamespaces, ConsistOf("kube-public")),
						WithTransform(GetOwnerReferences, ConsistOf([][]metav1.OwnerReference{ownerReferences})),
					)))
				})
			})

			When("secret is removed", func() {
				It("should have an owner table with the secret refs", func() {
					owners := map[interface{}]interface{}{}
					controller.owners.Range(func(key, value interface{}) bool { owners[key] = value; return true })

					expectedOwners := map[interface{}]interface{}{
						owner.UID: types.NamespacedName{Namespace: owner.Namespace, Name: owner.Name},
						types.NamespacedName{Namespace: owner.Namespace, Name: owner.Name}: owner.UID,
					}
					Expect(owners).Should(Equal(expectedOwners))
				})

				It("should remove the secret", func() {
					Expect(client.Delete(gocontext.TODO(), owner)).To(Succeed())
					res, err := controller.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: owner.Namespace, Name: owner.Name}})
					Expect(err).Should(Succeed())
					Expect(res).Should(Equal(reconcile.Result{}))
				})

				It("should have an empty owner table", func() {
					owners := map[interface{}]interface{}{}
					controller.owners.Range(func(key, value interface{}) bool { owners[key] = value; return true })
					Expect(owners).Should(BeEmpty())
				})
			})
		})
	})
	_ = Describe("Reconcile secrets with '"+namespaceSelectorAnnotation+"' annotation", func() {
		var client client.Client
		var controller reconcileSecret

		owner, ownerReferences := setupSpecs(&client, &controller)

		When("when is invalid", func() {
			owner := owner.DeepCopy()

			assertNoReplicatedSecret := func() {
				secrets := &corev1.SecretList{}
				Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
				Expect(secrets.Items).Should(HaveLen(1))
				Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), HaveLen(0)))
			}

			When("when there is no selector", func() {
				owner.Annotations = map[string]string{namespaceSelectorAnnotation: ""}
				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
				It("should contain one secret & zero replicated secret", assertNoReplicatedSecret)
			})

			When("when the selector is invalid", func() {
				owner.Annotations = map[string]string{namespaceSelectorAnnotation: "select-nothing"}
				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
				It("should contain one secret & zero replicated secret", assertNoReplicatedSecret)
			})

			When("when the selector doesn't match", func() {
				owner.Annotations = map[string]string{namespaceSelectorAnnotation: "need-shared-secret=true"}
				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
				It("should contain one secret & zero replicated secret", assertNoReplicatedSecret)
			})
		})

		When("selector is 'need-shared-secret=true'", func() {
			owner := owner.DeepCopy()
			owner.Annotations = map[string]string{namespaceSelectorAnnotation: "need-shared-secret=true"}

			It("must update 'kube-public' namespace", func() {
				Expect(client.Update(gocontext.TODO(), &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "kube-public",
						Labels: map[string]string{"need-shared-secret": "true"},
					},
				}))
			})

			It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
			It("should contain two secrets & one replicated secrets", func() {
				secrets := &corev1.SecretList{}
				Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
				Expect(secrets.Items).Should(HaveLen(2))
				Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), And(
					WithTransform(GetNames, ConsistOf(owner.Name)),
					WithTransform(GetNamespaces, ConsistOf("kube-public")),
					WithTransform(GetOwnerReferences, ConsistOf([][]metav1.OwnerReference{ownerReferences})),
				)))
			})

			When("annotations are updated", func() {
				owner.Annotations["foo"] = "bar"

				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
				It("should contain two replicated secrets with the new annotation", func() {
					expectedAnnotations := map[string]string{"foo": "bar"}
					secrets := &corev1.SecretList{}

					Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(2))
					Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), And(
						WithTransform(GetNames, ConsistOf(owner.Name)),
						WithTransform(GetNamespaces, ConsistOf("kube-public")),
						WithTransform(GetAnnotations, ConsistOf(expectedAnnotations)),
					)))
				})
			})

			When("labels are updated", func() {
				owner.Labels = map[string]string{"foo": "bar"}

				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, &controller, owner) })
				It("should contain two replicated secrets with the new annotation", func() {
					expectedLabels := map[string]string{"foo": "bar"}
					secrets := &corev1.SecretList{}

					Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(2))
					Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), And(
						WithTransform(GetNames, ConsistOf(owner.Name)),
						WithTransform(GetNamespaces, ConsistOf("kube-public")),
						WithTransform(GetLabels, ConsistOf(expectedLabels)),
					)))
				})
			})

			Context("with 'kube-public' namespace ignored", func() {
				controller := controller.DeepCopy()
				controller.ignoredNamespaces = []string{"kube-public"}

				It("should reconcile the updated secret", func() { reconcileUpdatedSecret(client, controller, owner) })
				It("should contain one secret & no replicated secret", func() {
					//TODO: removing mechanism is not implemented
					Skip("removing mechanism is not implemented")

					secrets := &corev1.SecretList{}
					Expect(client.List(gocontext.TODO(), secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(1))
					Expect(secrets.Items).Should(WithTransform(IgnoreOwner(*owner), HaveLen(0)))
				})
			})
		})
	})
)
