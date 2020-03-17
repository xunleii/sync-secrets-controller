package controller

import (
	"flag"
	"io/ioutil"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func init() {
	fset := flag.NewFlagSet("ignore_logs", flag.ExitOnError)
	klog.SetOutput(ioutil.Discard)
	klog.InitFlags(fset)

	_ = fset.Parse([]string{"-logtostderr=false", "-alsologtostderr=false", "-stderrthreshold=10"})
}

var (
	IgnoreOwner = func(owner corev1.Secret) func([]corev1.Secret) []corev1.Secret {
		return func(secrets []corev1.Secret) []corev1.Secret {
			return funk.Filter(secrets, func(secret corev1.Secret) bool { return secret.UID != owner.UID }).([]corev1.Secret)
		}
	}
	GetNames = func(secrets []corev1.Secret) []string {
		return funk.Map(secrets, func(secret corev1.Secret) string { return secret.Name }).([]string)
	}
	GetNamespaces = func(secrets []corev1.Secret) []string {
		return funk.Map(secrets, func(secret corev1.Secret) string { return secret.Namespace }).([]string)
	}
	GetAnnotations = func(secrets []corev1.Secret) []map[string]string {
		return funk.Map(secrets, func(secret corev1.Secret) map[string]string { return secret.Annotations }).([]map[string]string)
	}
	GetLabels = func(secrets []corev1.Secret) []map[string]string {
		return funk.Map(secrets, func(secret corev1.Secret) map[string]string { return secret.Labels }).([]map[string]string)
	}
	GetOwnerReferences = func(secrets []corev1.Secret) [][]metav1.OwnerReference {
		return funk.Map(secrets, func(secret corev1.Secret) []metav1.OwnerReference { return secret.OwnerReferences }).([][]metav1.OwnerReference)
	}
)

func TestNewController(t *testing.T) {
	controller := NewController(":8888", ":9999", []string{"kube-system"})

	T := NewWithT(t)
	T.Expect(controller).ToNot(BeNil())
	T.Expect(controller.metricsBindAddress).Should(Equal(":8888"))
	T.Expect(controller.healthProbeBindAddress).Should(Equal(":9999"))
	T.Expect(controller.ignoredNamespaces).Should(Equal([]string{"kube-system"}))
}

func TestReconcileSecret(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}
