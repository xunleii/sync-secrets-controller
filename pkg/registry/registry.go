package registry

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

type (
	// Registry keeps in memory all states of the managed secrets and their
	// owned secrets.
	// - A managed secret is a secret watched by the controller, which must
	//   be synced.
	// - An owned secret is a secret created by the controller, which is a
	//   copy of a managed secret.
	Registry struct {
		// secretsByUID maps all managed secrets with their UID
		secretsByUID map[types.UID]*Secret
		// secretsByOwnedSecretName maps all managed secrets with their owned secret NamespacedNames
		secretsByOwnedSecretName map[types.NamespacedName]*Secret
		// ownedSecretsBySecretUID maps all owned secrets with the owner secret UID
		ownedSecretsBySecretUID map[types.UID][]types.NamespacedName

		mx sync.RWMutex
	}
	Secret struct {
		types.NamespacedName
		types.UID
	}
)

// New creates a new registry
func New() *Registry {
	return &Registry{
		secretsByUID:             map[types.UID]*Secret{},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{},
		ownedSecretsBySecretUID:  map[types.UID][]types.NamespacedName{},
		mx:                       sync.RWMutex{},
	}
}

// Secrets returns all register secret's names.
func (r *Registry) Secrets() []types.NamespacedName {
	r.mx.RLock()
	defer r.mx.RUnlock()

	var secrets []types.NamespacedName
	for _, secret := range r.secretsByUID {
		secrets = append(secrets, secret.NamespacedName)
	}
	return secrets
}

// SecretWithName returns a registered secret with the given name, or nil
// if doesn't exists.
func (r *Registry) SecretWithName(name types.NamespacedName) *Secret {
	r.mx.RLock()
	defer r.mx.RUnlock()

	for _, secret := range r.secretsByUID {
		if secret.NamespacedName == name {
			return secret
		}
	}
	return nil
}

// SecretWithUID returns a registered secret with the given UID, or nil
// if doesn't exists.
func (r *Registry) SecretWithUID(uid types.UID) *Secret {
	r.mx.RLock()
	defer r.mx.RUnlock()

	secret := r.secretsByUID[uid]
	return secret
}

// SecretWithName returns a registered secret with the given NamespacedName,
// or nil if doesn't exists.
func (r *Registry) SecretWithOwnedSecretName(ownedSecretName types.NamespacedName) *Secret {
	r.mx.RLock()
	defer r.mx.RUnlock()

	secret := r.secretsByOwnedSecretName[ownedSecretName]
	return secret
}

// Secrets returns all register owned secret's names.
func (r *Registry) OwnedSecretsWithUID(uid types.UID) []types.NamespacedName {
	r.mx.RLock()
	defer r.mx.RUnlock()

	return r.ownedSecretsBySecretUID[uid]
}

// secretWithName returns a registered secret with the given name, or nil
// if doesn't exists.
func (r *Registry) secretWithName(name string) *Secret {
	r.mx.RLock()
	defer r.mx.RUnlock()

	for _, secret := range r.secretsByUID {
		if secret.Name == name {
			return secret
		}
	}
	return nil
}

// RegisterSecret adds a new secret to the registry.
func (r *Registry) RegisterSecret(name types.NamespacedName, uid types.UID) error {
	if r.SecretWithUID(uid) != nil {
		return nil //NOTE: ignore if it already exists
	}

	if r.secretWithName(name.Name) != nil {
		return SecretNameAlreadyExistsErr(name.Name)
	}

	r.mx.Lock()
	secret := &Secret{
		NamespacedName: name,
		UID:            uid,
	}
	r.secretsByUID[uid] = secret
	r.ownedSecretsBySecretUID[uid] = []types.NamespacedName{}
	r.mx.Unlock()

	return nil
}

// UnregisterSecret removes a secret from the registry.
func (r *Registry) UnregisterSecret(uid types.UID) error {
	secret := r.SecretWithUID(uid)
	if secret == nil {
		return SecretNotFoundErr{field: "UID", value: string(uid)}
	}

	r.mx.Lock()
	delete(r.secretsByUID, uid)
	for _, name := range r.ownedSecretsBySecretUID[uid] {
		delete(r.secretsByOwnedSecretName, name)
	}
	delete(r.ownedSecretsBySecretUID, uid)
	r.mx.Unlock()

	return nil
}

// RegisterOwnedSecret adds a new owned secret to the registry.
func (r *Registry) RegisterOwnedSecret(managerUID types.UID, name types.NamespacedName) error {
	if r.SecretWithOwnedSecretName(name) != nil {
		return nil //NOTE: ignore if it already exists
	}

	secret := r.SecretWithUID(managerUID)
	if secret == nil {
		return SecretNotFoundErr{field: "UID", value: string(managerUID)}
	}

	r.mx.Lock()
	r.secretsByOwnedSecretName[name] = secret
	r.ownedSecretsBySecretUID[managerUID] = append(r.ownedSecretsBySecretUID[managerUID], name)
	r.mx.Unlock()

	return nil
}

// UnregisterOwnedSecret removes an owned secret to the registry.
func (r *Registry) UnregisterOwnedSecret(name types.NamespacedName) error {
	secret := r.SecretWithOwnedSecretName(name)
	if secret == nil {
		return SecretNotFoundErr{field: "owned secret name", value: name.String()}
	}

	r.mx.Lock()
	delete(r.secretsByOwnedSecretName, name)

	n := -1
	ownedList := r.ownedSecretsBySecretUID[secret.UID]
	for i, owned := range ownedList {
		if owned == name {
			n = i
			break
		}
	}
	if n > -1 {
		ownedList[n] = ownedList[len(ownedList)-1]
		r.ownedSecretsBySecretUID[secret.UID] = ownedList[:len(ownedList)-1]
	}
	r.mx.Unlock()

	return nil
}
