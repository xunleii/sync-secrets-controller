package controller_test

import (
	gocontext "context"
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/xunleii/sync-secrets-operator/pkg/controller"
)

var _ = Describe("SynchronizeSecret", func() {
	var kube client.Client
	var context *controller.Context
	var uid = uuid.NewUUID()

	BeforeEach(func() {
		kube = fake.NewFakeClientWithScheme(scheme.Scheme)
		context = controller.NewContext(gocontext.TODO(), kube)

		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}})).To(Succeed())
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}})).To(Succeed())
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-lease"}})).To(Succeed())
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})).To(Succeed())

		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "alpha", Labels: map[string]string{"sync": "secret"}}})).To(Succeed())
		Expect(kube.Create(context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "bravo", Labels: map[string]string{"sync": "secret"}}})).To(Succeed())
	})

	It("should not synchronize without annotation", func() {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "fake-secret", Namespace: "default", UID: uid},
			Type:       "Opaque",
			Data: map[string][]byte{
				"username": []byte(base64.StdEncoding.EncodeToString([]byte("my-app"))),
				"password": []byte(base64.StdEncoding.EncodeToString([]byte("39528$vdg7Jb"))),
			},
		}

		// SynchronizeSecret returns nil in this error case in order to ignore unmanaged secrets
		Expect(controller.SynchronizeSecret(context, secret)).Should(Succeed())

		var secrets corev1.SecretList
		Expect(kube.List(context, &secrets)).To(Succeed())
		Expect(secrets.Items).Should(BeEmpty())
	})

	It("should not synchronize with both annotations", func() {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fake-secret", Namespace: "default", UID: uid,
				Annotations: map[string]string{
					controller.AllNamespacesAnnotation:     "true",
					controller.NamespaceSelectorAnnotation: "sync=secret",
				},
			},
			Type: "Opaque",
			Data: map[string][]byte{
				"username": []byte(base64.StdEncoding.EncodeToString([]byte("my-app"))),
				"password": []byte(base64.StdEncoding.EncodeToString([]byte("39528$vdg7Jb"))),
			},
		}

		Expect(controller.SynchronizeSecret(context, secret)).Should(And(
			HaveOccurred(),
			BeAssignableToTypeOf(controller.AnnotationError{}),
		))

		var secrets corev1.SecretList
		Expect(kube.List(context, &secrets)).To(Succeed())
		Expect(secrets.Items).Should(BeEmpty())
	})

	When("secret has annotation '"+controller.AllNamespacesAnnotation+"'", func() {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fake-secret", Namespace: "default", UID: uid,
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

		When("annotation is 'true'", func() {
			When("secret's namespace is ignored", func() {
				IgnoredNamespaces := []string{secret.Namespace}

				It("should not synchronize", func() {
					context.IgnoredNamespaces = IgnoredNamespaces

					Expect(controller.SynchronizeSecret(context, secret)).Should(Succeed())

					var secrets corev1.SecretList
					Expect(kube.List(context, &secrets)).To(Succeed())
					Expect(secrets.Items).Should(BeEmpty())
				})
			})

			When("'kube-*' namespaces are ignored", func() {
				IgnoredNamespaces := []string{"kube-system", "kube-lease", "kube-public"}

				It("should synchronize secret only on other namespaces", func() {
					context.IgnoredNamespaces = IgnoredNamespaces

					Expect(controller.SynchronizeSecret(context, secret)).Should(Succeed())

					var secrets corev1.SecretList
					Expect(kube.List(context, &secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(2))
				})
			})

			When("no namespace is ignored", func() {
				It("should synchronize secret", func() {
					Expect(controller.SynchronizeSecret(context, secret)).Should(Succeed())

					var secrets corev1.SecretList
					Expect(kube.List(context, &secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(5))
				})

				It("should synchronize secret on targeted namespace only ('kube-public' & 'alpha')", func() {
					Expect(controller.SynchronizeSecret(context, secret, "kube-public", "alpha")).Should(Succeed())

					var secrets corev1.SecretList
					Expect(kube.List(context, &secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(2))
				})

				When("secret is already synced and secret is updated", func() {
					It("should update owned secrets", func() {
						const annotation = "new-annotation"
						Expect(controller.SynchronizeSecret(context, secret)).To(Succeed())

						countAnnotations := func(secrets []corev1.Secret) int {
							var hasAnnotation int

							for _, secret := range secrets {
								if _, exists := secret.Annotations[annotation]; exists {
									hasAnnotation++
								}
							}
							return hasAnnotation
						}
						var secrets corev1.SecretList
						Expect(kube.List(context, &secrets)).To(Succeed())
						Expect(secrets.Items).Should(HaveLen(5))
						Expect(secrets.Items).Should(WithTransform(countAnnotations, Equal(0)))

						secret.Annotations[annotation] = ""
						Expect(controller.SynchronizeSecret(context, secret)).To(Succeed())
						Expect(kube.List(context, &secrets)).To(Succeed())
						Expect(secrets.Items).Should(HaveLen(5))
						Expect(secrets.Items).Should(WithTransform(countAnnotations, Equal(5)))
					})
				})
			})
		})

		When("annotation is not 'true'", func() {
			secret := *secret.DeepCopy()
			secret.Annotations[controller.AllNamespacesAnnotation] = "false"

			It("should not synchronize secret", func() {
				Expect(controller.SynchronizeSecret(context, secret)).Should(And(
					HaveOccurred(),
					BeAssignableToTypeOf(controller.AnnotationError{}),
				))

				var secrets corev1.SecretList
				Expect(kube.List(context, &secrets)).To(Succeed())
				Expect(secrets.Items).Should(BeEmpty())
			})
		})
	})

	When("secret has annotation '"+controller.NamespaceSelectorAnnotation+"'", func() {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fake-secret", Namespace: "default", UID: uid,
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

		When("annotation is valid", func() {
			When("'alpha' namespaces are ignored", func() {
				IgnoredNamespaces := []string{"alpha"}

				It("should synchronize secret only on 'bravo'", func() {
					context.IgnoredNamespaces = IgnoredNamespaces

					Expect(controller.SynchronizeSecret(context, secret)).Should(Succeed())

					var secrets corev1.SecretList
					Expect(kube.List(context, &secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(1))
				})
			})

			When("no namespace is ignored", func() {
				It("should synchronize secret only on 'alpha' & 'bravo'", func() {
					Expect(controller.SynchronizeSecret(context, secret)).Should(Succeed())

					var secrets corev1.SecretList
					Expect(kube.List(context, &secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(2))
				})

				It("should synchronize secret on targeted ('kube-public' & 'alpha') and selected ('alpha' & 'bravo') only", func() {
					Expect(controller.SynchronizeSecret(context, secret, "kube-public", "alpha")).Should(Succeed())

					var secrets corev1.SecretList
					Expect(kube.List(context, &secrets)).To(Succeed())
					Expect(secrets.Items).Should(HaveLen(1))
				})
			})
		})

		When("annotation is invalid", func() {
			secret := *secret.DeepCopy()
			secret.Annotations[controller.NamespaceSelectorAnnotation] = "invalid-selector!!!"

			It("should not synchronize secret", func() {
				Expect(controller.SynchronizeSecret(context, secret)).Should(And(
					HaveOccurred(),
					BeAssignableToTypeOf(controller.AnnotationError{}),
				))

				var secrets corev1.SecretList
				Expect(kube.List(context, &secrets)).To(Succeed())
				Expect(secrets.Items).Should(BeEmpty())
			})
		})
	})
})
