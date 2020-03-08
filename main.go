package main

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("example-controller")

func main() {
	logf.SetLogger(zap.Logger(false))
	entryLog := log.WithName("entrypoint")

	// Setup a Manager
	entryLog.Info("setting up manager")
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	// Setup a new controller to reconcile ReplicaSets
	entryLog.Info("Setting up controller")
	c, err := controller.New("foo-controller", mgr, controller.Options{
		Reconciler: &reconcileSecret{client: mgr.GetClient(), log: log.WithName("reconciler")},
	})
	if err != nil {
		entryLog.Error(err, "unable to set up individual controller")
		os.Exit(1)
	}

	// Watch Secret and enqueue Secret object key
	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}); err != nil {
		entryLog.Error(err, "unable to watch Secret")
		os.Exit(1)
	}

	// Watch Secret and enqueue Secret object key
	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{OwnerType:&corev1.Secret{}}); err != nil {
		entryLog.Error(err, "unable to watch Secret")
		os.Exit(1)
	}

	entryLog.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

// reconcileSecret reconciles ReplicaSets
type reconcileSecret struct {
	// client can be used to retrieve objects from the APIServer.
	client client.Client
	log    logr.Logger
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileSecret{}

func (r *reconcileSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// set up a convenient log object so we don't have to type request over and over again
	log := r.log.WithValues("request", request)

	srt := &corev1.Secret{}
	err := r.client.Get(context.TODO(), request.NamespacedName, srt)
	if errors.IsNotFound(err) {
		// ignore
		return reconcile.Result{}, nil
	} else if err != nil {
		log.Error(err, "Not work")
		return reconcile.Result{}, err
	}

	annotations := srt.Annotations
	allNamespaces, anExists := annotations["export.secret.sync.klst.pw/all-namespaces"]
	namespaceSelector, nsExists := annotations["export.secret.sync.klst.pw/namespace-selector"]

	if !anExists && !nsExists {
		// ignore
		return reconcile.Result{}, nil
	} else if anExists && nsExists {
		log.Error(nil, "export.secret.sync.klst.pw/all-namespaces and export.secret.sync.klst.pw/namespace-selector cannot be used together")
		return reconcile.Result{}, nil
	}

	secretCopy := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:        srt.Name,
			Labels:      srt.Labels,
			Annotations: srt.Annotations,
			OwnerReferences: []v1.OwnerReference{{
				APIVersion: srt.APIVersion,
				Kind:       srt.Kind,
				Name:       srt.Name,
				UID:        srt.UID,
			}},
		},
		Data: srt.Data,
	}
	delete(secretCopy.Annotations, "export.secret.sync.klst.pw/all-namespaces")
	delete(secretCopy.Annotations, "export.secret.sync.klst.pw/namespace-selector")

	log.Info("secret modification requested", "ns", request.Namespace, "name", request.Name)
	var options []client.ListOption
	if anExists && allNamespaces == "true" {
		log.Info("duplication secret on all namespaces")
	} else {
		selector, err := labels.Parse(namespaceSelector)
		if err != nil {
			log.Error(err, "Oops, invalid selector")
		}

		options = append(options, client.MatchingLabelsSelector{Selector: selector})
	}

	nsList := &corev1.NamespaceList{}
	_ = r.client.List(context.TODO(), nsList, options...)
	//log.Error(err, "namespaces", "namespaceList", nsList)

	for _, namespace := range nsList.Items {
		if namespace.Name == srt.Namespace {
			continue
		}

		secret := secretCopy.DeepCopy()
		secret.Namespace = namespace.Name
		err := r.client.Create(context.TODO(), secret)
		if errors.IsAlreadyExists(err) {
			err = r.client.Update(context.TODO(), secret)
		}

		if err != nil {
			log.Error(err, "Oops, failed to create/update it... ignore it", "namespace", namespace.Name)
		}
	}

	//// Fetch the ReplicaSet from the cache
	//rs := &appsv1.ReplicaSet{}
	//err := r.client.Get(context.TODO(), request.NamespacedName, rs)
	//if errors.IsNotFound(err) {
	//	log.Error(nil, "Could not find ReplicaSet")
	//	return reconcile.Result{}, nil
	//}
	//
	//if err != nil {
	//	log.Error(err, "Could not fetch ReplicaSet")
	//	return reconcile.Result{}, err
	//}
	//
	//// Print the ReplicaSet
	//log.Info("Reconciling ReplicaSet", "container name", rs.Spec.Template.Spec.Containers[0].Name)
	//
	//// Set the label if it is missing
	//if rs.Labels == nil {
	//	rs.Labels = map[string]string{}
	//}
	////if rs.Labels["hello"] == "world" {
	////	return reconcile.Result{}, nil
	////}
	//
	//// Update the ReplicaSet
	//delete(rs.Labels, "hello")
	//rs.Labels["hello"] = "world"
	//err = r.client.Update(context.TODO(), rs)
	//if err != nil {
	//	log.Error(err, "Could not write ReplicaSet")
	//	return reconcile.Result{}, err
	//}

	return reconcile.Result{}, nil
}
