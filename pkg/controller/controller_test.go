package controller

import (
	"context"
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/cucumber/messages-go/v10"
	kubernetes_ctx "github.com/xunleii/godog-kubernetes"
	"github.com/xunleii/godog-kubernetes/helpers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var opts = godog.Options{Output: colors.Colored(os.Stdout)}

func init() {
	godog.BindFlags("godog.", flag.CommandLine, &opts)

	// Disable klog outputs
	fset := flag.NewFlagSet("ignore_logs", flag.ExitOnError)
	klog.SetOutput(ioutil.Discard)
	klog.InitFlags(fset)
	_ = fset.Parse([]string{"-logtostderr=false", "-alsologtostderr=false", "-stderrthreshold=10"})
}

func TestMain(m *testing.M) {
	// Parse flags and prepare godog
	flag.Parse()
	opts.Paths = flag.Args()

	status := godog.TestSuite{
		Name:                "sync-secrets-controller",
		ScenarioInitializer: InitializeScenario,
		Options:             &opts,
	}.Run()

	// Run godog
	if st := m.Run(); st > status {
		status = st
	}
	os.Exit(status)
}

func InitializeScenario(s *godog.ScenarioContext) {
	featureContext, _ := kubernetes_ctx.NewFeatureContext(s, kubernetes_ctx.WithFakeClient(scheme.Scheme))

	var ctx *Context
	var reconcilers = map[string]reconcile.Reconciler{}

	s.BeforeScenario(func(*messages.Pickle) {
		ctx = NewContext(context.TODO(), featureContext.Client())
		reconcilers["secret"] = &SecretReconciler{ctx}
		reconcilers["owned secret"] = &OwnedSecretReconcilier{ctx}
		reconcilers["namespace"] = &NamespaceReconciler{ctx}
	})

	s.Step("nothing occurs", func() error { return nil })
	s.Step(
		`^the (secret|owned secret|namespace) reconciler reconciles '(`+kubernetes_ctx.RxNamespacedName+`)'$`,
		func(reconciler, name string) error {
			target, err := helpers.NamespacedNameFrom(name)
			if err != nil {
				return err
			}

			_, err = reconcilers[reconciler].Reconcile(reconcile.Request{NamespacedName: target})
			switch err.(type) {
			case NoAnnotationError:
				return nil
			default:
				return err
			}
		},
	)
	s.Step(
		`^the v1/Namespace '(.+)' is ignored by the reconciler$`,
		func(namespace string) error {
			ctx.IgnoredNamespaces = append(ctx.IgnoredNamespaces, namespace)
			return nil
		},
	)
	s.Step(
		`^the (label|annotation) '(.+)' is protected by the reconciler$`,
		func(_type string, field string) error {
			switch _type {
			case "label":
				ctx.ProtectedLabels = append(ctx.ProtectedLabels, field)
			case "annotation":
				ctx.ProtectedAnnotations = append(ctx.ProtectedAnnotations, field)
			}
			return nil
		},
	)
}
