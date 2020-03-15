package controller

import (
	gocontext "context"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	prefixAnnotation            = "secret.sync.klst.pw"
	allNamespacesAnnotation     = prefixAnnotation + "/all-namespaces"
	namespaceSelectorAnnotation = prefixAnnotation + "/namespace-selector"

	requeueAfter = 5 * time.Second
)

type (
	context struct {
		ignoredNamespaces []string
		owners            sync.Map
	}

	Controller struct {
		context
		metricsBindAddress     string
		healthProbeBindAddress string
	}
)

func NewController(metricsBindAddress, healthProbeBindAddress string, ignoredNamespaces []string) *Controller {
	return &Controller{
		context: context{
			ignoredNamespaces: ignoredNamespaces,
		},
		metricsBindAddress:     metricsBindAddress,
		healthProbeBindAddress: healthProbeBindAddress,
	}
}

func (c *Controller) Run(stop <-chan struct{}) {
	mgr, err := manager.New(kconfig.GetConfigOrDie(), manager.Options{
		MetricsBindAddress:     c.metricsBindAddress,
		HealthProbeBindAddress: c.healthProbeBindAddress,
		ReadinessEndpointName:  "/-/readyz",
		LivenessEndpointName:   "/-/healthz",
	})
	if err != nil {
		klog.Fatalf("Unable to set up overall controller manager: %s", err)
	}

	_ = mgr.AddReadyzCheck("readyz", func(req *http.Request) error { return nil })
	_ = mgr.AddHealthzCheck("healthz", func(req *http.Request) error { return nil })

	secretCtrl, err := controller.New("sync-secrets", mgr, controller.Options{
		Reconciler: &reconcileSecret{context: &c.context, client: mgr.GetClient()},
	})
	if err != nil {
		klog.Fatalf("Unable to set up individual controller (sync-secrets): %s", err)
	}

	err = secretCtrl.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		klog.Fatalf("Unable to watch %T: %s", &corev1.Secret{}, err)
	}

	ownedSecretCtrl, err := controller.New("sync-owned-secrets", mgr, controller.Options{
		Reconciler: &reconcileOwnedSecret{context: &c.context, client: mgr.GetClient()},
	})
	if err != nil {
		klog.Fatalf("Unable to set up individual controller (sync-owned-secrets): %s", err)
	}

	err = ownedSecretCtrl.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForOwner{OwnerType: &corev1.Secret{}},
		predicate.Funcs{
			// Ignore creation because it is managed by the secret owner
			CreateFunc:  func(event.CreateEvent) bool { return false },
			DeleteFunc:  func(event.DeleteEvent) bool { return true },
			UpdateFunc:  func(event.UpdateEvent) bool { return true },
			GenericFunc: func(event.GenericEvent) bool { return true },
		},
	)
	if err != nil {
		klog.Fatalf("Unable to watch owned %T: %s", &corev1.Secret{}, err)
	}

	err = mgr.Start(stop)
	if err != nil {
		klog.Fatalf("Failed to run overall controller manager: %s", err)
	}
}

func copySecret(client client.Client, owner *corev1.Secret, target types.NamespacedName) error {
	copy := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:        owner.Name,
			Namespace:   target.Namespace,
			Labels:      owner.Labels,
			Annotations: owner.Annotations,
			OwnerReferences: []v1.OwnerReference{{
				APIVersion: owner.APIVersion,
				Kind:       owner.Kind,
				Name:       owner.Name,
				UID:        owner.UID,
			}},
		},
		Data: owner.Data,
	}
	delete(copy.Annotations, allNamespacesAnnotation)
	delete(copy.Annotations, namespaceSelectorAnnotation)

	secret := &corev1.Secret{}
	err := client.Get(gocontext.TODO(), target, secret)
	if errors.IsNotFound(err) {
		klog.V(3).Infof("Secret %T %s doesn't exists and must be created", copy, target)
		err := client.Create(gocontext.TODO(), copy)
		if err != nil {
			klog.Errorf("Failed to create %T %s: %s... ignore", copy, target, err)
			return err
		}
		return nil
	} else if err != nil {
		klog.Errorf("Failed to fetch %T %s: %s... ignore", copy, target, err)
		return err
	}

	if !(len(secret.OwnerReferences) == 1 && secret.OwnerReferences[0].UID == owner.UID) {
		klog.Errorf("Secret %T %s is not owned by %s (%s)... ignore", secret, target, owner.Name, owner.UID)
		return nil
	}

	err = client.Update(gocontext.TODO(), copy)
	if err != nil {
		klog.Errorf("Failed to update %T %s: %s... ignore", secret, target, err)
		return err
	}
	return nil
}
