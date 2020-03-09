package controller

import (
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	prefixAnnotation            = "secret.sync.klst.pw"
	allNamespacesAnnotation     = prefixAnnotation + "/all-namespaces"
	namespaceSelectorAnnotation = prefixAnnotation + "/namespace-selector"

	requeueAfter = 15 * time.Second
)

type (
	context struct {
		ignoredNamespaces []string
		owners            sync.Map
	}

	Controller struct {
		context
	}
)

func NewController(ignoredNamespaces []string) *Controller {
	return &Controller{
		context: context{
			ignoredNamespaces: ignoredNamespaces,
		},
	}
}

func (c *Controller) Run(stop <-chan struct{}) {
	mgr, err := manager.New(kconfig.GetConfigOrDie(), manager.Options{
		//MetricsBindAddress:    "",
		//ReadinessEndpointName: "/-/readyz",
		//LivenessEndpointName:  "/-/healthz",
	})
	if err != nil {
		klog.Fatalf("unable to set up overall controller manager: %s", err)
	}

	//_ = mgr.AddReadyzCheck("readyz", func(req *http.Request) error { return nil })
	//_ = mgr.AddHealthzCheck("healthz", func(req *http.Request) error { return nil })

	secretCtrl, err := controller.New("sync-secrets", mgr, controller.Options{
		Reconciler: &reconcileSecrets{context: c.context, client: mgr.GetClient()},
	})
	if err != nil {
		klog.Fatalf("Unable to set up individual controller (sync-secrets): %s", err)
	}

	err = secretCtrl.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		klog.Fatalf("Unable to watch %T: %s", &corev1.Secret{}, err)
	}

	err = mgr.Start(stop)
	if err != nil {
		panic(err)
	}
}
