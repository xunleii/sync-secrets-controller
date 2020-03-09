package controller

import (
	gocontext "context"
	"strings"

	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconcileSecrets struct {
	context
	client client.Client
}

func (r *reconcileSecrets) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(gocontext.TODO(), req.NamespacedName, secret)
	if errors.IsNotFound(err) {
		klog.V(3).Infof("Failed to fetch %T %s: %s", secret, req.NamespacedName, err)
		klog.V(5).Infof("This error occurs when a secret is deleted")

		if uid, isManaged := r.owners.Load(req); isManaged {
			klog.V(4).Infof("Remove %T %s from owner table", secret, req)
			r.owners.Delete(req)
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
	case allExists == false && selectorExists == false:
		klog.V(5).Infof("Annotation not found on %T %s, request ignored", secret, req)
		// TODO(ani): remove synced secrets if req is in the owner table
		return reconcile.Result{}, nil
	case allExists == true && selectorExists == true:
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

	klog.V(4).Infof("Cache %T %s in the owner table", secret, req)
	klog.V(2).Infof("Sync %T %s on selected namespaces", secret, req)
	r.owners.LoadOrStore(req, secret.UID)
	r.owners.LoadOrStore(secret.UID, req)

	namespaces := &corev1.NamespaceList{}
	err = r.client.List(gocontext.TODO(), namespaces, options...)
	if err != nil {
		klog.Errorf("Failed to list %T %s: %s... retry after %s", namespaces, req, err, requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}

	owner := v1.OwnerReference{
		APIVersion: secret.APIVersion,
		Kind:       secret.Kind,
		Name:       secret.Name,
		UID:        secret.UID,
	}
	syncedSecret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:            secret.Name,
			Labels:          secret.Labels,
			Annotations:     secret.Annotations,
			OwnerReferences: []v1.OwnerReference{owner},
		},
		Data: secret.Data,
	}
	delete(syncedSecret.Annotations, allNamespacesAnnotation)
	delete(syncedSecret.Annotations, namespaceSelectorAnnotation)

	for _, namespace := range namespaces.Items {
		if funk.ContainsString(ignoredNamespaces, namespace.Name) {
			continue
		}

		secret := &corev1.Secret{}
		objKey := types.NamespacedName{Namespace: namespace.Name, Name: syncedSecret.Name}

		err := r.client.Get(gocontext.TODO(), objKey, secret)
		if errors.IsNotFound(err) {
			secret := syncedSecret.DeepCopy()
			secret.Namespace = namespace.Name

			klog.V(3).Infof("Secret %T %s doesn't exists and must be created", secret, objKey)
			err := r.client.Create(gocontext.TODO(), secret)
			if err != nil {
				klog.Errorf("Failed to create %T %s: %s... ignore", secret, objKey, err)
			}
			continue
		} else if err != nil {
			klog.Errorf("Failed to fetch %T %s: %s... ignore", secret, objKey, err)
			continue
		}

		if !(len(secret.OwnerReferences) == 1 && secret.OwnerReferences[0] == owner) {
			klog.Errorf("Secret %T %s is not owned by %s (%s)... ignore", secret, objKey, owner.Name, owner.UID)
			continue
		}

		secret = syncedSecret.DeepCopy()
		secret.Namespace = namespace.Name
		err = r.client.Update(gocontext.TODO(), secret)
		if err != nil {
			klog.Errorf("Failed to update %T %s: %s... ignore", secret, objKey, err)
			continue
		}
	}

	return reconcile.Result{}, nil
}
