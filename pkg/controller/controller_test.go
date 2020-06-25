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
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kfc "github.com/xunleii/sync-secrets-controller/pkg/controller/kubernetes_feature_context"
)

var opt = godog.Options{Output: colors.Colored(os.Stdout)}

func init() {
	godog.BindFlags("godog.", flag.CommandLine, &opt)

	// Disable klog outputs
	fset := flag.NewFlagSet("ignore_logs", flag.ExitOnError)
	klog.SetOutput(ioutil.Discard)
	klog.InitFlags(fset)
	_ = fset.Parse([]string{"-logtostderr=false", "-alsologtostderr=false", "-stderrthreshold=10"})
}

func TestMain(m *testing.M) {
	// Parse flags and prepare godog
	flag.Parse()
	opt.Paths = flag.Args()
	status := godog.RunWithOptions("godogs", func(s *godog.Suite) {
		FeatureContext(s)
	}, opt)

	// Run godog
	if st := m.Run(); st > status {
		status = st
	}
	os.Exit(status)
}

func FeatureContext(s *godog.Suite) {
	featureContext, _ := kfc.FeatureContext(s, kfc.FakeClient, kfc.UseCustomGC(kfc.ManualGC))

	s.Step("nothing occurs", func() error { return nil })

	var ctx *Context
	var reconcilers = map[string]reconcile.Reconciler{}
	s.BeforeScenario(func(*messages.Pickle) {
		ctx = NewContext(context.TODO(), featureContext.Client)
		reconcilers["secret"] = &SecretReconciler{ctx}
		reconcilers["owned secret"] = &OwnedSecretReconcilier{ctx}
		reconcilers["namespace"] = &NamespaceReconciler{ctx}
	})

	s.Step(
		`^the (secret|owned secret|namespace) reconciler reconciles '(`+kfc.RxNamespacedName+`)'$`,
		func(reconciler, name string) error {
			target, err := kfc.NamespacedNameFrom(name)
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
