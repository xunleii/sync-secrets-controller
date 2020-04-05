package controller

import (
	gocontext "context"
	"encoding/base64"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/xunleii/sync-secrets-operator/pkg/registry"
)

func TestSynchronizeSecret(t *testing.T) {
	ctx := context{
		Context:           gocontext.TODO(),
		registry:          registry.New(),
		client:            fake.NewFakeClientWithScheme(scheme.Scheme),
		ignoredNamespaces: []string{"kube-system"},
	}
	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default", Annotations: map[string]string{allNamespacesAnnotation: "true"}},
		Data:       map[string][]byte{"owner": []byte(base64.StdEncoding.EncodeToString([]byte("the-owner-value")))},
		Type:       "Opaque",
	}

	// TODO: test sync
	//   - with no annotation
	//   - with both annotation
	//   - with all namespace annotation
	//   - with namespace selector annotation
	//   - with resync
}
