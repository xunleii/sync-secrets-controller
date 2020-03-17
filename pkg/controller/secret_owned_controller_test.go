package controller

import (
	gocontext "context"
	"encoding/base64"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	_ = Describe("Reconcile owned secrets", func() {
		var (
			owner = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shared-secret", Namespace: "default", UID: uuid.NewUUID(),
					Annotations: map[string]string{allNamespacesAnnotation: "true"},
				},
				Data: map[string][]byte{"owner": []byte(base64.StdEncoding.EncodeToString([]byte("the-owner-value")))},
			}
			client     = fake.NewFakeClientWithScheme(scheme.Scheme)
			controller = &reconcileOwnedSecret{context: &context{owners: &sync.Map{}}, client: client}
		)

		It("must create default namespaces", func() {
			namespaces := []string{"kube-system", "kube-public", "default"}
			for _, namespace := range namespaces {
				Expect(client.Create(gocontext.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).
					To(Succeed())
			}
		})
		It("must create original secret", func() { Expect(client.Create(gocontext.TODO(), owner)).To(Succeed()) })
		It("must create owned secrets", func() {
			controller := reconcileSecret{context: &context{owners: controller.owners}, client: client}

			req := reconcile.Request{NamespacedName: types.NamespacedName{Name: owner.Name, Namespace: owner.Namespace}}
			res, err := controller.Reconcile(req)
			Expect(err).To(Succeed())
			Expect(res).To(Equal(reconcile.Result{}))
		})

		var ownedSecret corev1.Secret
		It("must get kube-public/shared-secret", func() {
			req := types.NamespacedName{Name: owner.Name, Namespace: "kube-public"}
			Expect(client.Get(gocontext.TODO(), req, &ownedSecret)).To(Succeed())
		})

		When("an owned secret is modified", func() {
			ownedSecret.Labels = map[string]string{"added-label": "will-be-ignored"}

			It("must update kube-public/shared-secret", func() {
				Expect(client.Update(gocontext.TODO(), &ownedSecret)).To(Succeed())

				ownedSecret := corev1.Secret{}
				req := types.NamespacedName{Name: owner.Name, Namespace: "kube-public"}
				Expect(client.Get(gocontext.TODO(), req, &ownedSecret)).To(Succeed())

				Expect(ownedSecret.Labels).To(Equal(map[string]string{"added-label": "will-be-ignored"}))
			})
			It("should reconcile kube-public/shared-secret", func() {
				req := reconcile.Request{NamespacedName: types.NamespacedName{Name: owner.Name, Namespace: "kube-public"}}
				res, err := controller.Reconcile(req)
				Expect(err).Should(Succeed())
				Expect(res).Should(Equal(reconcile.Result{}))

				ownedSecret := corev1.Secret{}
				Expect(client.Get(gocontext.TODO(), req.NamespacedName, &ownedSecret)).To(Succeed())

				Expect(ownedSecret.Labels).Should(BeEmpty())
			})
		})

		When("when an owned secret is deleted", func() {
			It("must delete kube-public/shared-secret", func() {
				req := types.NamespacedName{Name: owner.Name, Namespace: "kube-public"}
				Expect(client.Delete(gocontext.TODO(), &ownedSecret)).To(Succeed())
				Expect(errors.IsNotFound(client.Get(gocontext.TODO(), req, &ownedSecret))).To(BeTrue())
			})
			It("should reconcile kube-public/shared-secret", func() {
				Skip("removing mechanism is not implemented")

				req := reconcile.Request{NamespacedName: types.NamespacedName{Name: owner.Name, Namespace: "kube-public"}}
				res, err := controller.Reconcile(req)
				Expect(err).Should(Succeed())
				Expect(res).Should(Equal(reconcile.Result{}))

				Expect(client.Get(gocontext.TODO(), req.NamespacedName, &ownedSecret)).To(Succeed())
				Expect(ownedSecret.Labels).Should(BeEmpty())
			})
		})
	})
)
