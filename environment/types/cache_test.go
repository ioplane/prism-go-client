package types_test

import (
	"net/url"
	"testing"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedClientParams(t *testing.T) {
	// Test that the interface can be implemented
	t.Run("interface implementation", func(t *testing.T) {
		testURL, err := url.Parse("https://test.example.com")
		require.NoError(t, err)

		params := &testCachedClientParams{
			endpoint: types.ManagementEndpoint{
				ApiCredentials: types.ApiCredentials{
					Username: fixtureUsername,
					Password: fixturePassword,
				},
				Address: testURL,
			},
			key: "test-key",
		}

		assert.Equal(t, params.endpoint, params.ManagementEndpoint())
		assert.Equal(t, "test-key", params.Key())
	})
}

// testCachedClientParams is a test implementation of CachedClientParams.
type testCachedClientParams struct {
	endpoint types.ManagementEndpoint
	key      string
}

func (t *testCachedClientParams) ManagementEndpoint() types.ManagementEndpoint {
	return t.endpoint
}

func (t *testCachedClientParams) Key() string {
	return t.key
}
