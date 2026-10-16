package local

import (
	"net/url"
	"testing"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	fixtureValueLength  = 10
	fixtureAPIKeyLength = 32
)

func TestLocal(t *testing.T) {
	matcher := gomega.NewWithT(t)
	endpoint := rand.String(fixtureValueLength)
	username := rand.String(fixtureValueLength)
	password := rand.String(fixtureValueLength)
	prov := &provider{}
	val, err := prov.Get(nil, "Foo")
	matcher.Expect(val).To(gomega.BeNil())
	matcher.Expect(err).To(gomega.BeIdenticalTo(types.ErrNotFound))

	t.Setenv(userEnv, username)
	t.Setenv(passwordEnv, password)
	t.Setenv(categoriesEnv, "k8s/foo,project/bar")

	t.Setenv(endpointEnv, endpoint)

	managementEndpoint, err := prov.GetManagementEndpoint(nil)
	matcher.Expect(err).To(gomega.Succeed())

	expectedAddr := "https://" + endpoint + ":9440"
	matcher.Expect(managementEndpoint.Address.String()).To(gomega.BeIdenticalTo(expectedAddr))

	t.Setenv(endpointEnv, "not valid")

	_, err = prov.GetManagementEndpoint(nil)
	matcher.Expect(err).NotTo(gomega.Succeed())

	t.Setenv(endpointEnv, endpoint)

	managementEndpoint, err = prov.GetManagementEndpoint(nil)
	matcher.Expect(err).To(gomega.Succeed())
	matcher.Expect(managementEndpoint.Address.String()).To(gomega.BeIdenticalTo(expectedAddr))
	matcher.Expect(managementEndpoint.ApiCredentials).To(gomega.BeIdenticalTo(types.ApiCredentials{
		Username: username,
		Password: password,
		KeyPair:  "",
		APIKey:   "",
	}))

	val, err = prov.Get(nil, types.CategoriesKey)
	matcher.Expect(err).NotTo(gomega.HaveOccurred())
	matcher.Expect(val).To(gomega.BeEquivalentTo([]string{"k8s/foo", "project/bar"}))

	const marker = "synthetic-provider-url-secret-marker"
	for _, invalid := range []string{"fixture:" + marker + "@%41.invalid", "%41.invalid?token=" + marker} {
		t.Setenv(endpointEnv, invalid)

		managementEndpoint, err := prov.GetManagementEndpoint(nil)
		require.Nil(t, managementEndpoint)
		require.Error(t, err)
		require.NotContains(t, err.Error(), marker)

		var cause *url.Error
		require.ErrorAs(t, err, &cause)
		require.Equal(t, "https://"+invalid+":9440", cause.URL)
		require.Equal(t, "parse", cause.Op)
	}
}

func TestLocalRejectsMixedAuth(t *testing.T) {
	matcher := gomega.NewWithT(t)
	endpoint := rand.String(fixtureValueLength)
	username := rand.String(fixtureValueLength)
	password := rand.String(fixtureValueLength)
	apiKey := rand.String(fixtureAPIKeyLength)
	prov := &provider{}

	t.Setenv(endpointEnv, endpoint)
	t.Setenv(userEnv, username)
	t.Setenv(passwordEnv, password)
	t.Setenv(apiKeyEnv, apiKey)

	_, err := prov.GetManagementEndpoint(nil)
	matcher.Expect(err).NotTo(gomega.Succeed())
	require.EqualError(t, err, "basic auth (username/password) and API key cannot be set simultaneously")
}

func TestLocalRejectsIncompleteBasicAuthWhenAPIKeyMissing(t *testing.T) {
	matcher := gomega.NewWithT(t)
	endpoint := rand.String(fixtureValueLength)
	username := rand.String(fixtureValueLength)
	prov := &provider{}

	t.Setenv(endpointEnv, endpoint)
	t.Setenv(userEnv, username)
	t.Setenv(passwordEnv, "")
	t.Setenv(apiKeyEnv, "")

	_, err := prov.GetManagementEndpoint(nil)
	matcher.Expect(err).NotTo(gomega.Succeed())
	require.EqualError(t, err, "either username and password, or an API key must be set in provider configuration")
}
