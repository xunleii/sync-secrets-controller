package registry

import (
	"fmt"
)

type SecretNameAlreadyExistsErr string

func (e SecretNameAlreadyExistsErr) Error() string {
	return fmt.Sprintf("secret name '%s' already exists; this can create conflicts during synchronization", string(e))
}

type SecretNotFoundErr struct {
	field string
	value string
}

func (e SecretNotFoundErr) Error() string {
	return fmt.Sprintf("secret with the given %s '%s' not found", e.field, e.value)
}
