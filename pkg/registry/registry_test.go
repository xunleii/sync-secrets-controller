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
		secretsByUID: map[types.UID]*Secret{secret.UID: secret},
		secretsByOwnedSecretName: map[types.NamespacedName]*Secret{
			types.NamespacedName{Namespace: "kube-system", Name: "test"}: secret,
			types.NamespacedName{Namespace: "kube-public", Name: "test"}: secret,
			types.NamespacedName{Namespace: "custom", Name: "test"}:      secret,
		},
		ownedSecretsBySecretUID: map[types.UID][]types.NamespacedName{
			secret.UID: {
				types.NamespacedName{Namespace: "kube-system", Name: "test"},
				types.NamespacedName{Namespace: "kube-public", Name: "test"},
				types.NamespacedName{Namespace: "custom", Name: "test"},
			},
		},
	}
)

func TestRegistry_New(t *testing.T) {
	registry := New()

	assert.NotNil(t, registry.secretsByOwnedSecretName)
	assert.NotNil(t, registry.secretsByUID)
	assert.NotNil(t, registry.ownedSecretsBySecretUID)
}

func TestRegistry_Secrets(t *testing.T) {
	assert.Contains(t, registry.Secrets(), types.NamespacedName{Namespace: "default", Name: "test"})
}

func TestRegistry_SecretWithName(t *testing.T) {
	tests := []struct {
		name   string
		arg    types.NamespacedName
		expect *Secret
	}{
		{"WithValidName", secret.NamespacedName, secret},
		{"WithInvalidName", types.NamespacedName{Namespace: "kube-public", Name: "test"}, nil}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := registry.SecretWithName(tt.arg)
			assert.Equal(t, tt.expect, actual)
		})
	}
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

func TestRegistry_SecretWithOwnedSecretName(t *testing.T) {
	tests := []struct {
		name   string
		arg    types.NamespacedName
		expect *Secret
	}{
		{"WithValidOwnedSecretName_One", types.NamespacedName{Namespace: "kube-system", Name: "test"}, secret},
		{"WithValidOwnedSecretName_Two", types.NamespacedName{Namespace: "custom", Name: "test"}, secret},
		{"WithInvalidOwnedSecretName", types.NamespacedName{Namespace: "default", Name: "test"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := registry.SecretWithOwnedSecretName(tt.arg)
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func TestRegistry_OwnedSecretWithUID(t *testing.T) {
	tests := []struct {
		name   string
		arg    types.UID
		expect []types.NamespacedName
	}{
		{"WithValidUID", secret.UID, []types.NamespacedName{
			{Namespace: "kube-system", Name: "test"},
			{Namespace: "kube-public", Name: "test"},
			{Namespace: "custom", Name: "test"},
		}},
		{"WithInvalidUID", "00000000-0000-0000-0000-000000000000", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := registry.OwnedSecretsWithUID(tt.arg)
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func TestRegistry_RegisterSecret(t *testing.T) {
	registry := New()

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
				types.NamespacedName{Namespace: "kube-system", Name: "test"},
				"294db320-e51e-480f-bc11-95cad45e3841",
			),
			"secret name 'test' already exists; this can create conflicts during synchronization",
		)
	})

	t.Run("VerifyInternalState", func(t *testing.T) {
		assert.Len(t, registry.secretsByUID, 1)
		assert.Contains(t, registry.secretsByUID, secret.UID)

		assert.Empty(t, registry.secretsByOwnedSecretName)

		assert.Len(t, registry.ownedSecretsBySecretUID, 1)
		assert.Contains(t, registry.ownedSecretsBySecretUID, secret.UID)
		assert.Empty(t, registry.ownedSecretsBySecretUID[secret.UID])
	})
}

func TestRegistry_RegisterSecretAsync(t *testing.T) {
	registry := New()

	var secrets []Secret
	for i := 0; i < 1e4; i++ {
		secrets = append(secrets, Secret{
			NamespacedName: types.NamespacedName{Namespace: "default", Name: fmt.Sprintf("test-%d", i)},
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
	registry := New()

	// preflight checks
	require.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))
	require.NoError(t, registry.RegisterOwnedSecret(secret.UID, types.NamespacedName{Namespace: "kube-system", Name: "test"}))
	require.Contains(t, registry.secretsByUID, secret.UID)
	require.Contains(t, registry.secretsByOwnedSecretName, types.NamespacedName{Namespace: "kube-system", Name: "test"})
	require.Contains(t, registry.ownedSecretsBySecretUID, secret.UID)

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
		assert.Empty(t, registry.secretsByOwnedSecretName)
		assert.Empty(t, registry.ownedSecretsBySecretUID)
	})
}

func TestRegistry_RegisterOwnedSecret(t *testing.T) {
	registry := New()

	// preflight checks
	require.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))
	require.Contains(t, registry.secretsByUID, secret.UID)
	require.Empty(t, registry.secretsByOwnedSecretName)
	require.Contains(t, registry.ownedSecretsBySecretUID, secret.UID)
	require.Empty(t, registry.ownedSecretsBySecretUID[secret.UID])

	t.Run("WithInvalidSecretUID", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.RegisterOwnedSecret(
				"00000000-0000-0000-0000-000000000000",
				types.NamespacedName{Namespace: "kube-system", Name: "test"},
			),
			"secret with the given UID '00000000-0000-0000-0000-000000000000' not found",
		)
	})

	t.Run("WithNewOwnedSecret", func(t *testing.T) {
		assert.NoError(t, registry.RegisterOwnedSecret(secret.UID, types.NamespacedName{Namespace: "kube-system", Name: "test"}))
	})

	t.Run("WithRegisteredOwnedSecret", func(t *testing.T) {
		//NOTE: register a registered secret is silently ignored
		assert.NoError(t, registry.RegisterOwnedSecret(secret.UID, types.NamespacedName{Namespace: "kube-system", Name: "test"}))
	})

	t.Run("VerifyInternalState", func(t *testing.T) {
		assert.Len(t, registry.secretsByUID, 1)
		assert.Contains(t, registry.secretsByUID, secret.UID)

		assert.Len(t, registry.secretsByOwnedSecretName, 1)
		assert.Contains(t, registry.secretsByOwnedSecretName, types.NamespacedName{Namespace: "kube-system", Name: "test"})

		assert.Len(t, registry.ownedSecretsBySecretUID, 1)
		assert.Contains(t, registry.ownedSecretsBySecretUID, secret.UID)
		assert.Contains(t, registry.ownedSecretsBySecretUID[secret.UID], types.NamespacedName{Namespace: "kube-system", Name: "test"})
	})
}

func TestRegistry_RegisterOwnedSecretAsync(t *testing.T) {
	registry := New()
	require.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))

	var ownedSecrets []types.NamespacedName
	for i := 0; i < 1e4; i++ {
		ownedSecrets = append(ownedSecrets, types.NamespacedName{Namespace: "default", Name: fmt.Sprintf("test-%d", i)})
	}

	errg := errgroup.Group{}
	for _, s := range ownedSecrets {
		s := s
		errg.Go(func() error { return registry.RegisterOwnedSecret(secret.UID, s) })
	}
	assert.NoError(t, errg.Wait())
}

func TestRegistry_UnregisterOwnedSecret(t *testing.T) {
	registry := New()

	// preflight checks
	require.NoError(t, registry.RegisterSecret(secret.NamespacedName, secret.UID))
	require.NoError(t, registry.RegisterOwnedSecret(secret.UID, types.NamespacedName{Namespace: "kube-system", Name: "test"}))
	require.Contains(t, registry.secretsByUID, secret.UID)
	require.Contains(t, registry.secretsByOwnedSecretName, types.NamespacedName{Namespace: "kube-system", Name: "test"})
	assert.Contains(t, registry.ownedSecretsBySecretUID, secret.UID)
	assert.Contains(t, registry.ownedSecretsBySecretUID[secret.UID], types.NamespacedName{Namespace: "kube-system", Name: "test"})

	t.Run("WithUnknownOwnedSecret", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.UnregisterOwnedSecret(types.NamespacedName{Namespace: "kube-pulic", Name: "test"}),
			"secret with the given owned secret name '"+types.NamespacedName{Namespace: "kube-pulic", Name: "test"}.String()+"' not found",
		)
	})

	t.Run("WithRegisteredSecret", func(t *testing.T) {
		assert.NoError(t, registry.UnregisterOwnedSecret(types.NamespacedName{Namespace: "kube-system", Name: "test"}))
	})

	t.Run("WithUnregisteredSecret", func(t *testing.T) {
		assert.EqualError(
			t,
			registry.UnregisterOwnedSecret(types.NamespacedName{Namespace: "kube-system", Name: "test"}),
			"secret with the given owned secret name '"+types.NamespacedName{Namespace: "kube-system", Name: "test"}.String()+"' not found",
		)
	})

	t.Run("VerifyInternalState", func(t *testing.T) {
		assert.Len(t, registry.secretsByUID, 1)
		assert.Contains(t, registry.secretsByUID, secret.UID)

		assert.Empty(t, registry.secretsByOwnedSecretName)

		assert.Len(t, registry.ownedSecretsBySecretUID, 1)
		assert.Contains(t, registry.ownedSecretsBySecretUID, secret.UID)
		assert.Empty(t, registry.ownedSecretsBySecretUID[secret.UID])
	})
}
