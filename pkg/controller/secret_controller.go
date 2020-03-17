package controller

import (
	gocontext "context"
	"strings"

	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconcileSecret struct {
	*context
	client client.Client
}

func (r *reconcileSecret) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(gocontext.TODO(), req.NamespacedName, secret)
	if errors.IsNotFound(err) {
		klog.V(3).Infof("Failed to fetch %T %s: %s", secret, req.NamespacedName, err)
		klog.V(5).Infof("This error occurs when a secret is deleted")

		if uid, isManaged := r.owners.Load(req.NamespacedName); isManaged {
			klog.V(4).Infof("Remove %T %s from owner table", secret, req)
			r.owners.Delete(req.NamespacedName)
			r.owners.Delete(uid)
		}
		return reconcile.Result{}, nil
	} else if err != nil {
		klog.Errorf("Failed to fetch %T %s: %s... retry after %s", secret, req, err, requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}
	ignoredNamespaces := append(r.ignoredNamespaces, secret.Namespace)

	onAllNamespaces, allExists := secret.Annotations[allNamespacesAnnotation]
	namespaceSelector, selectorExists := secret.Annotations[namespaceSelectorAnnotation]

	var options []client.ListOption
	switch {
	case !allExists && !selectorExists:
		klog.V(5).Infof("Annotation not found on %T %s, request ignored", secret, req)
		// TODO(ani): remove synced secrets if req is in the owner table
		return reconcile.Result{}, nil
	case allExists && selectorExists:
		klog.Errorf("Invalid annotations on %T %s: annotations %s and %s cannot be used together", secret, req, allNamespacesAnnotation, namespaceSelectorAnnotation)
		// TODO(ani): remove synced secrets if req is in the owner table
		return reconcile.Result{}, nil
	case allExists:
		if strings.ToLower(onAllNamespaces) != "true" {
			klog.V(3).Infof("Annotation %s on %T %s is not 'true', request ignored", allNamespacesAnnotation, secret, req)
			// TODO(ani): remove synced secrets if req is in the owner table
			return reconcile.Result{}, nil
		}
		klog.V(3).Infof("Sync %T %s on all namespaces except %v", secret, req, r.ignoredNamespaces)
	case selectorExists:
		selector, err := labels.Parse(namespaceSelector)
		if err != nil {
			klog.Errorf("Failed to parse %s on %T %s: %s", namespaceSelectorAnnotation, secret, req, err)
			// TODO(ani): remove synced secrets if req is in the owner table
			return reconcile.Result{}, nil
		}

		options = append(options, client.MatchingLabelsSelector{Selector: selector})
		klog.V(3).Infof("Sync %T %s on all namespaces validating %s, except %v", secret, req, selector, r.ignoredNamespaces)
	}

	if _, alreadyCached := r.owners.Load(req.NamespacedName); !alreadyCached {
		klog.V(4).Infof("Cache %T %s in the owner table", secret, req)
		r.owners.LoadOrStore(req.NamespacedName, secret.UID)
		r.owners.LoadOrStore(secret.UID, req.NamespacedName)
	}
	klog.V(2).Infof("Sync %T %s on selected namespaces", secret, req)

	namespaces := &corev1.NamespaceList{}
	err = r.client.List(gocontext.TODO(), namespaces, options...)
	if err != nil {
		klog.Errorf("Failed to list %T %s: %s... retry after %s", namespaces, req, err, requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}

	for _, namespace := range namespaces.Items {
		if funk.ContainsString(ignoredNamespaces, namespace.Name) {
			continue
		}

		_ = copySecret(r.client, secret, types.NamespacedName{Namespace: namespace.Name, Name: secret.Name})
	}

	return reconcile.Result{}, nil
}

func (r *reconcileSecret) DeepCopy() *reconcileSecret {
	return &reconcileSecret{
		context: r.context.DeepCopy(),
		client:  r.client,
	}
}
