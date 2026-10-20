package types_test

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixtureScheme             = "https"
	fixtureHost               = "test.example.com"
	fixtureHashLength         = 64
	fixtureUsername           = "testuser"
	fixturePassword           = "testpass"
	fixtureAPIKey             = "test-api-key"
	fixtureAltUsername        = "test-user"
	fixtureAltPassword        = "test-password"
	fixtureMissingAuthMessage = "either username and password, or an API key must be set in provider configuration"
	fixtureRegionKey          = "region"
	fixtureRegionValue        = "us-west-1"
)

func TestApiCredentials(t *testing.T) {
	t.Run("JSON serialization", func(t *testing.T) {
		fixtureCredentials := types.ApiCredentials{
			Username: fixtureUsername,
			Password: fixturePassword,
			KeyPair:  "testkeypair",
			APIKey:   fixtureAPIKey,
		}

		data, err := json.Marshal(fixtureCredentials)
		require.NoError(t, err)

		var decoded types.ApiCredentials

		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, fixtureCredentials.Username, decoded.Username)
		assert.Equal(t, fixtureCredentials.Password, decoded.Password)
		assert.Equal(t, fixtureCredentials.KeyPair, decoded.KeyPair)
		assert.Equal(t, fixtureCredentials.APIKey, decoded.APIKey)
	})

	t.Run("JSON serialization with empty fields", func(t *testing.T) {
		fixtureCredentials := types.ApiCredentials{}

		data, err := json.Marshal(fixtureCredentials)
		require.NoError(t, err)

		var decoded types.ApiCredentials

		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Empty(t, decoded.Username)
		assert.Empty(t, decoded.Password)
		assert.Empty(t, decoded.KeyPair)
		assert.Empty(t, decoded.APIKey)
	})

	t.Run("Validate", checkCredentialValidation)
}

func checkCredentialValidation(t *testing.T) {
	t.Helper()

	tests := []struct {
		name        string
		creds       *types.ApiCredentials
		wantErr     bool
		errContains string
	}{
		{name: "nil credentials", creds: nil, wantErr: true, errContains: "credentials must not be nil"},
		{name: "valid basic auth", creds: &types.ApiCredentials{Username: fixtureAltUsername, Password: fixtureAltPassword}, wantErr: false, errContains: ""},
		{name: "valid API key", creds: &types.ApiCredentials{APIKey: fixtureAPIKey}, wantErr: false, errContains: ""},
		{
			name:    "rejects API key and basic auth together",
			creds:   &types.ApiCredentials{Username: fixtureAltUsername, Password: fixtureAltPassword, APIKey: fixtureAPIKey},
			wantErr: true, errContains: "basic auth (username/password) and API key cannot be set simultaneously",
		},
		{
			name: "rejects empty credentials", creds: &types.ApiCredentials{}, wantErr: true,
			errContains: fixtureMissingAuthMessage,
		},
		{
			name: "rejects username without password", creds: &types.ApiCredentials{Username: fixtureAltUsername}, wantErr: true,
			errContains: fixtureMissingAuthMessage,
		},
		{
			name: "rejects password without username", creds: &types.ApiCredentials{Password: fixtureAltPassword}, wantErr: true,
			errContains: fixtureMissingAuthMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.creds.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errContains)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestManagementEndpoint(t *testing.T) {
	t.Run("JSON serialization", func(t *testing.T) {
		testURL, err := url.Parse("https://test.example.com")
		require.NoError(t, err)

		endpoint := types.ManagementEndpoint{
			Username:              fixtureUsername,
			Password:              fixturePassword,
			Address:               testURL,
			Insecure:              true,
			AdditionalTrustBundle: "test-bundle",
		}

		data, err := json.Marshal(endpoint)
		require.NoError(t, err)

		var decoded types.ManagementEndpoint

		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, endpoint.Username, decoded.Username)
		assert.Equal(t, endpoint.Password, decoded.Password)
		assert.Equal(t, endpoint.Address.String(), decoded.Address.String())
		assert.Equal(t, endpoint.Insecure, decoded.Insecure)
		assert.Equal(t, endpoint.AdditionalTrustBundle, decoded.AdditionalTrustBundle)
	})

	t.Run("JSON serialization with nil address", func(t *testing.T) {
		endpoint := types.ManagementEndpoint{
			Username: fixtureUsername,
			Address:  nil,
			Insecure: false,
		}

		data, err := json.Marshal(endpoint)
		require.NoError(t, err)

		var decoded types.ManagementEndpoint

		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, endpoint.Username, decoded.Username)
		assert.Nil(t, decoded.Address)
		assert.Equal(t, endpoint.Insecure, decoded.Insecure)
	})

	t.Run("JSON serialization with API key authentication", func(t *testing.T) {
		testURL, err := url.Parse("https://test.example.com")
		require.NoError(t, err)

		endpoint := types.ManagementEndpoint{
			APIKey:                fixtureAPIKey,
			Address:               testURL,
			Insecure:              false,
			AdditionalTrustBundle: "test-bundle",
		}

		data, err := json.Marshal(endpoint)
		require.NoError(t, err)

		var decoded types.ManagementEndpoint

		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, endpoint.APIKey, decoded.APIKey)
		assert.Empty(t, decoded.Username)
		assert.Empty(t, decoded.Password)
		assert.Equal(t, endpoint.Address.String(), decoded.Address.String())
		assert.Equal(t, endpoint.Insecure, decoded.Insecure)
		assert.Equal(t, endpoint.AdditionalTrustBundle, decoded.AdditionalTrustBundle)
	})
}

func TestTopology(t *testing.T) {
	t.Run("topology map operations", func(t *testing.T) {
		topology := types.Topology{
			fixtureRegionKey: fixtureRegionValue,
			"zone":           "us-west-1a",
			"rack":           "rack-1",
		}

		assert.Equal(t, fixtureRegionValue, topology[fixtureRegionKey])
		assert.Equal(t, "us-west-1a", topology["zone"])
		assert.Equal(t, "rack-1", topology["rack"])

		// Test modification
		topology["rack"] = "rack-2"
		assert.Equal(t, "rack-2", topology["rack"])

		// Test addition
		topology["datacenter"] = "dc-1"
		assert.Equal(t, "dc-1", topology["datacenter"])
	})

	t.Run("empty topology", func(t *testing.T) {
		topology := types.Topology{}
		assert.Empty(t, topology)
		value, exists := topology["nonexistent"]
		assert.False(t, exists)
		assert.Empty(t, value)
	})
}

type hashPair struct {
	name   string
	first  types.ManagementEndpoint
	second types.ManagementEndpoint
}

func hashDifferenceCases() []hashPair {
	address := &url.URL{Scheme: fixtureScheme, Host: fixtureHost}

	return []hashPair{
		{
			name: "different endpoints produce different hashes",
			first: types.ManagementEndpoint{
				Username: fixtureUsername, Password: fixturePassword,
				Address: &url.URL{Scheme: fixtureScheme, Host: "test1.example.com"},
			},
			second: types.ManagementEndpoint{
				Username: fixtureUsername, Password: fixturePassword,
				Address: &url.URL{Scheme: fixtureScheme, Host: "test2.example.com"},
			},
		},
		{
			name:   "hash changes with credential changes",
			first:  types.ManagementEndpoint{Username: "user1", Password: "pass1", Address: address},
			second: types.ManagementEndpoint{Username: "user2", Password: "pass2", Address: address},
		},
		{
			name:  "hash changes with API key changes",
			first: types.ManagementEndpoint{APIKey: "api-key-1", Address: address}, second: types.ManagementEndpoint{APIKey: "api-key-2", Address: address},
		},
		{
			name: "hash differs between basic auth and API key auth",
			first: types.ManagementEndpoint{
				Username: fixtureUsername, Password: fixturePassword,
				Address: address,
			},
			second: types.ManagementEndpoint{APIKey: fixtureAPIKey, Address: address},
		},
		{
			name: "hash changes with insecure flag",
			first: types.ManagementEndpoint{
				Username: fixtureUsername, Password: fixturePassword,
				Address: address, Insecure: false,
			},
			second: types.ManagementEndpoint{
				Username: fixtureUsername, Password: fixturePassword,
				Address: address, Insecure: true,
			},
		},
		{
			name: "hash changes with trust bundle",
			first: types.ManagementEndpoint{
				Username: fixtureUsername, Password: fixturePassword,
				Address: address, AdditionalTrustBundle: "bundle1",
			},
			second: types.ManagementEndpoint{
				Username: fixtureUsername, Password: fixturePassword,
				Address: address, AdditionalTrustBundle: "bundle2",
			},
		},
	}
}

func checkHashGeneration(t *testing.T) {
	t.Helper()

	endpoint := types.ManagementEndpoint{
		Username: fixtureUsername, Password: fixturePassword,
		Address: &url.URL{Scheme: fixtureScheme, Host: fixtureHost}, Insecure: true,
	}
	hash, err := endpoint.GetHash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, fixtureHashLength)
}

func checkHashConsistency(t *testing.T) {
	t.Helper()

	endpoint := types.ManagementEndpoint{
		Username: fixtureUsername, Password: fixturePassword,
		Address: &url.URL{Scheme: fixtureScheme, Host: fixtureHost}, Insecure: true,
	}
	first, err := endpoint.GetHash()
	require.NoError(t, err)
	second, err := endpoint.GetHash()
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestGetManagementEndpointHash(t *testing.T) {
	t.Run("successful hash generation", checkHashGeneration)
	t.Run("hash consistency", checkHashConsistency)

	for _, test := range hashDifferenceCases() {
		t.Run(test.name, func(t *testing.T) {
			first, err := test.first.GetHash()
			require.NoError(t, err)
			second, err := test.second.GetHash()
			require.NoError(t, err)
			assert.NotEqual(t, first, second)
		})
	}
}

func TestConstants(t *testing.T) {
	t.Run("CategoriesKey constant", func(t *testing.T) {
		assert.Equal(t, "categories", types.CategoriesKey)
	})

	t.Run("ErrNotFound error", func(t *testing.T) {
		assert.Equal(t, "environment key not found", types.ErrNotFound.Error())
	})
}

func TestEnvironmentInterface(t *testing.T) {
	// Test that the interface can be implemented
	t.Run("interface implementation", func(t *testing.T) {
		env := &testEnvironment{}

		topology := types.Topology{
			fixtureRegionKey: fixtureRegionValue,
			"zone":           "us-west-1a",
		}

		endpoint, err := env.GetManagementEndpoint(topology)
		require.NoError(t, err)
		assert.NotNil(t, endpoint)

		value, err := env.Get(topology, "test-key")
		require.NoError(t, err)
		assert.Equal(t, "test-value", value)
	})
}

// testEnvironment is a test implementation of Environment.
type testEnvironment struct{}

func (*testEnvironment) GetManagementEndpoint(_ types.Topology) (*types.ManagementEndpoint, error) {
	testURL := &url.URL{Scheme: fixtureScheme, Host: fixtureHost}

	return &types.ManagementEndpoint{
		Username: fixtureUsername,
		Password: fixturePassword,
		Address:  testURL,
	}, nil
}

func (*testEnvironment) Get(_ types.Topology, _ string) (any, error) {
	return "test-value", nil
}

func TestProviderInterface(t *testing.T) {
	// Test that Provider interface extends Environment
	t.Run("interface composition", func(t *testing.T) {
		provider := &testProvider{}

		topology := types.Topology{
			fixtureRegionKey: fixtureRegionValue,
		}

		// Test Environment methods
		endpoint, err := provider.GetManagementEndpoint(topology)
		require.NoError(t, err)
		assert.NotNil(t, endpoint)

		value, err := provider.Get(topology, "test-key")
		require.NoError(t, err)
		assert.Equal(t, "test-value", value)
	})
}

// testProvider is a test implementation of Provider.
type testProvider struct{}

func (*testProvider) GetManagementEndpoint(_ types.Topology) (*types.ManagementEndpoint, error) {
	testURL := &url.URL{Scheme: fixtureScheme, Host: fixtureHost}

	return &types.ManagementEndpoint{
		Username: fixtureUsername,
		Password: fixturePassword,
		Address:  testURL,
	}, nil
}

func (*testProvider) Get(_ types.Topology, _ string) (any, error) {
	return "test-value", nil
}
