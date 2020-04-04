package registry

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

type (
	// Registry keeps in memory all states of the managed secretsByUID and their
	// owned secretsByUID.
	// - A managed secret is a secret watched by the controller, which must
	//   be synced.
	// - An owned secret is a secret created by the controller, which is a
	//   copy of a managed secret.
	Registry struct {
		// secretsByUID maps all managed secretsByUID with their UID
		secretsByUID map[types.UID]*Secret
		// secretsByName maps all managed secretsByUID with their NamespacedNames
		secretsByName map[types.NamespacedName]*Secret
		// secretsByOwnedSecretName maps all managed secret with their owned secret NamespacedNames
		secretsByOwnedSecretName map[types.NamespacedName]*Secret

		mx sync.RWMutex
	}
	Secret struct {
		types.NamespacedName
		types.UID
	}
)

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
func (r *Registry) SecretWithName(name types.NamespacedName) *Secret {
	r.mx.RLock()
	defer r.mx.RUnlock()

	secret := r.secretsByName[name]
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
	r.secretsByName[name] = secret
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
	delete(r.secretsByName, secret.NamespacedName)
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
	r.mx.Unlock()

	return nil
}
