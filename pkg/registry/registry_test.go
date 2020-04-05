package registry

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"
)

var (
	secret = &Secret{
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "test",
		},
		UID: "ee29a220-3db7-4e08-9f7f-8a1045b2d110",
	}
	registry = &Registry{
		secretsByUID:  map[types.UID]*Secret{secret.UID: secret},
		secretsByName: map[types.NamespacedName]*Secret{secret.NamespacedName: secret},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{
			types.NamespacedName{Namespace: "kube-system", Name: "test"}: secret,
			types.NamespacedName{Namespace: "kube-public", Name: "test"}: secret,
			types.NamespacedName{Namespace: "custom", Name: "test"}:      secret,
		},
	}
)

func TestRegistry_New(t *testing.T) {
	registry := New()

	assert.NotNil(t, registry.secretsByOwnedSecretName)
	assert.NotNil(t, registry.secretsByUID)
	assert.NotNil(t, registry.secretsByName)
}

func TestRegistry_SecretWithUID(t *testing.T) {
	tests := []struct {
		name   string
		arg    types.UID
		expect *Secret
	}{
		{"WithValidUID", secret.UID, secret},
		{"WithInvalidUID", "00000000-0000-0000-0000-000000000000", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := registry.SecretWithUID(tt.arg)
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func TestRegistry_SecretWithName(t *testing.T) {
	tests := []struct {
		name   string
		arg    types.NamespacedName
		expect *Secret
	}{
		{"WithValidName", secret.NamespacedName, secret},
		{"WithInvalidName", types.NamespacedName{"kube-public", "test"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := registry.SecretWithName(tt.arg)
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func TestRegistry_SecretWithOwnedSecretName(t *testing.T) {
	tests := []struct {
		name   string
		arg    types.NamespacedName
		expect *Secret
	}{
		{"WithValidOwnedSecretName_One", types.NamespacedName{"kube-system", "test"}, secret},
		{"WithValidOwnedSecretName_Two", types.NamespacedName{"custom", "test"}, secret},
		{"WithInvalidOwnedSecretName", types.NamespacedName{"default", "test"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := registry.SecretWithOwnedSecretName(tt.arg)
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func TestRegistry_RegisterSecret(t *testing.T) {
	registry := &Registry{
		secretsByUID:             map[types.UID]*Secret{},
		secretsByName:            map[types.NamespacedName]*Secret{},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{},
	}

	t.Run("WithNewSecret", func(t *testing.T) {
		assert.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))
	})

	t.Run("WithRegisteredSecret", func(t *testing.T) {
		//NOTE: register a registered secret is silently ignored
		assert.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))
	})

	t.Run("WithNewSecret_WithNameAlreadyExists", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.RegisterSecret(
				types.NamespacedName{"kube-system", "test"},
				"294db320-e51e-480f-bc11-95cad45e3841",
			),
			"secret name 'test' already exists; this can create conflicts during synchronization",
		)
	})

	t.Run("VerifyInternalState", func(t *testing.T) {
		assert.Len(t, registry.secretsByUID, 1)
		assert.Contains(t, registry.secretsByUID, secret.UID)

		assert.Len(t, registry.secretsByName, 1)
		assert.Contains(t, registry.secretsByName, secret.NamespacedName)

		assert.Empty(t, registry.secretsByOwnedSecretName)
	})
}

func TestRegistry_RegisterSecretAsync(t *testing.T) {
	registry := &Registry{
		secretsByUID:             map[types.UID]*Secret{},
		secretsByName:            map[types.NamespacedName]*Secret{},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{},
	}

	var secrets []Secret
	for i := 0; i < 1e4; i++ {
		secrets = append(secrets, Secret{
			NamespacedName: types.NamespacedName{"default", fmt.Sprintf("test-%d", i)},
			UID:            types.UID(uuid.New().String()),
		})
	}

	errg := errgroup.Group{}
	for _, s := range secrets {
		s := s
		errg.Go(func() error { return registry.RegisterSecret(s.NamespacedName, s.UID) })
	}
	assert.NoError(t, errg.Wait())
}

func TestRegistry_UnregisterSecret(t *testing.T) {
	registry := &Registry{
		secretsByUID:             map[types.UID]*Secret{},
		secretsByName:            map[types.NamespacedName]*Secret{},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{},
	}

	// preflight checks
	require.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))
	require.Contains(t, registry.secretsByUID, secret.UID)
	require.Contains(t, registry.secretsByName, secret.NamespacedName)
	require.Empty(t, registry.secretsByOwnedSecretName)

	t.Run("WithUnknownSecret", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.UnregisterSecret("00000000-0000-0000-0000-000000000000"),
			"secret with the given UID '00000000-0000-0000-0000-000000000000' not found",
		)
	})

	t.Run("WithRegisteredSecret", func(t *testing.T) {
		assert.NoError(t, registry.UnregisterSecret(secret.UID))
	})

	t.Run("WithUnregisteredSecret", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.UnregisterSecret(secret.UID),
			"secret with the given UID '"+string(secret.UID)+"' not found",
		)
	})

	t.Run("VerifyInternalState", func(t *testing.T) {
		assert.Empty(t, registry.secretsByUID)
		assert.Empty(t, registry.secretsByName)
		assert.Empty(t, registry.secretsByOwnedSecretName)
	})
}

func TestRegistry_RegisterOwnedSecret(t *testing.T) {
	registry := &Registry{
		secretsByUID:             map[types.UID]*Secret{},
		secretsByName:            map[types.NamespacedName]*Secret{},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{},
	}

	// preflight checks
	require.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))
	require.Contains(t, registry.secretsByUID, secret.UID)
	require.Contains(t, registry.secretsByName, secret.NamespacedName)
	require.Empty(t, registry.secretsByOwnedSecretName)

	t.Run("WithInvalidSecretUID", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.RegisterOwnedSecret(
				"00000000-0000-0000-0000-000000000000",
				types.NamespacedName{"kube-system", "test"},
			),
			"secret with the given UID '00000000-0000-0000-0000-000000000000' not found",
		)
	})

	t.Run("WithNewOwnedSecret", func(t *testing.T) {
		assert.NoError(t, registry.RegisterOwnedSecret(secret.UID, types.NamespacedName{"kube-system", "test"}))
	})

	t.Run("WithRegisteredOwnedSecret", func(t *testing.T) {
		//NOTE: register a registered secret is silently ignored
		assert.NoError(t, registry.RegisterOwnedSecret(secret.UID, types.NamespacedName{"kube-system", "test"}))
	})

	t.Run("VerifyInternalState", func(t *testing.T) {
		assert.Len(t, registry.secretsByUID, 1)
		assert.Contains(t, registry.secretsByUID, secret.UID)

		assert.Len(t, registry.secretsByName, 1)
		assert.Contains(t, registry.secretsByName, secret.NamespacedName)

		assert.Len(t, registry.secretsByOwnedSecretName, 1)
		assert.Contains(t, registry.secretsByOwnedSecretName, types.NamespacedName{"kube-system", "test"})
	})
}

func TestRegistry_RegisterOwnedSecretAsync(t *testing.T) {
	registry := &Registry{
		secretsByUID:             map[types.UID]*Secret{},
		secretsByName:            map[types.NamespacedName]*Secret{},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{},
	}
	require.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))

	var ownedSecrets []types.NamespacedName
	for i := 0; i < 1e4; i++ {
		ownedSecrets = append(ownedSecrets, types.NamespacedName{"default", fmt.Sprintf("test-%d", i)})
	}

	errg := errgroup.Group{}
	for _, s := range ownedSecrets {
		s := s
		errg.Go(func() error { return registry.RegisterOwnedSecret(secret.UID, s) })
	}
	assert.NoError(t, errg.Wait())
}

func TestRegistry_UnregisterOwnedSecret(t *testing.T) {
	registry := &Registry{
		secretsByUID:             map[types.UID]*Secret{},
		secretsByName:            map[types.NamespacedName]*Secret{},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{},
	}

	// preflight checks
	require.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))
	require.NoError(t, registry.RegisterOwnedSecret(secret.UID, types.NamespacedName{"kube-system", "test"}))
	require.Contains(t, registry.secretsByUID, secret.UID)
	require.Contains(t, registry.secretsByName, secret.NamespacedName)
	require.Contains(t, registry.secretsByOwnedSecretName, types.NamespacedName{"kube-system", "test"})

	t.Run("WithUnknownOwnedSecret", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.UnregisterOwnedSecret(types.NamespacedName{"kube-pulic", "test"}),
			"secret with the given owned secret name '"+types.NamespacedName{"kube-pulic", "test"}.String()+"' not found",
		)
	})

	t.Run("WithRegisteredSecret", func(t *testing.T) {
		assert.NoError(t, registry.UnregisterOwnedSecret(types.NamespacedName{"kube-system", "test"}))
	})

	t.Run("WithUnregisteredSecret", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.UnregisterOwnedSecret(types.NamespacedName{"kube-system", "test"}),
			"secret with the given owned secret name '"+types.NamespacedName{"kube-system", "test"}.String()+"' not found",
		)
	})

	t.Run("VerifyInternalState", func(t *testing.T) {
		assert.Len(t, registry.secretsByUID, 1)
		assert.Contains(t, registry.secretsByUID, secret.UID)

		assert.Len(t, registry.secretsByName, 1)
		assert.Contains(t, registry.secretsByName, secret.NamespacedName)

		assert.Empty(t, registry.secretsByOwnedSecretName)
	})
}
