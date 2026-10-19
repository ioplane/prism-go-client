package v4

import (
	"errors"
	"net/url"
	"strconv"
	"testing"
	"time"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithReadTimeoutRange(t *testing.T) {
	for _, test := range []struct {
		name    string
		minutes int
		want    time.Duration
		wantErr bool
	}{
		{name: "one minute", minutes: 1, want: time.Minute},
		{name: "largest whole minute", minutes: 153722867, want: 9223372020000000000},
		{name: "overflow", minutes: 153722868, want: 2 * time.Minute, wantErr: true},
		{name: "zero keeps previous setting", minutes: 0, want: 2 * time.Minute},
		{name: "negative keeps previous setting", minutes: -1, want: 2 * time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{readTimeout: 2 * time.Minute}
			option := WithReadTimeout(test.minutes)
			err := option(client)
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.want, client.readTimeout)

			var configured *Client
			var cause error
			created, err := NewV4Client(prismgoclient.Credentials{Endpoint: "fixture.invalid", APIKey: "fixture"},
				WithReadTimeout(2), func(c *Client) error {
					configured = c
					cause = option(c)
					return cause
				})
			if test.wantErr {
				require.Nil(t, created)
				require.Error(t, cause)
				require.ErrorIs(t, err, cause)
				require.EqualError(t, err, "failed to apply client option: "+cause.Error())
				require.Equal(t, &Client{readTimeout: test.want}, configured)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, created.VmApiInstance.ApiClient.ReadTimeout)
			require.Equal(t, test.want, created.ProtectionPoliciesApiInstance.ApiClient.ReadTimeout)
		})
	}
}

func TestNewV4ClientEndpointDiagnostics(t *testing.T) {
	const marker = "synthetic-endpoint-secret-marker"
	const overflowPort = "999999999999999999999999999999"
	for _, test := range []struct {
		name     string
		endpoint string
		urlError bool
	}{
		{name: "invalid host", endpoint: "fixture:" + marker + "@%41.invalid", urlError: true},
		{name: "invalid port", endpoint: "fixture:" + marker + "@fixture.invalid:invalid", urlError: true},
		{name: "overflow port", endpoint: "fixture:" + marker + "@fixture.invalid:" + overflowPort},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewV4Client(prismgoclient.Credentials{Endpoint: test.endpoint, APIKey: "fixture"})
			require.Nil(t, client)
			require.Error(t, err)
			require.NotContains(t, err.Error(), marker)
			require.NotContains(t, err.Error(), overflowPort)
			if test.urlError {
				var cause *url.Error
				require.ErrorAs(t, err, &cause)
				require.Equal(t, "https://"+test.endpoint, cause.URL)
			} else {
				var cause *strconv.NumError
				require.ErrorAs(t, err, &cause)
				require.Equal(t, overflowPort, cause.Num)
				require.ErrorIs(t, err, strconv.ErrRange)
			}
		})
	}
}

func TestNewV4ClientBasicAuth(t *testing.T) {
	// verifies positive client creation
	cred := prismgoclient.Credentials{
		URL:                "foo.com",
		Username:           "username",
		Password:           "password",
		Port:               "",
		Endpoint:           "0.0.0.0",
		Insecure:           true,
		FoundationEndpoint: "10.0.0.0",
		FoundationPort:     "8000",
		RequiredFields:     nil,
	}
	v4Client, err := NewV4Client(cred)
	assert.NoError(t, err)
	assert.NotNil(t, v4Client)
	assert.NotNil(t, v4Client.VmApiInstance)
	assert.NotNil(t, v4Client.ImagesApiInstance)
	assert.NotNil(t, v4Client.SubnetsApiInstance)
	assert.NotNil(t, v4Client.SubnetIPReservationApi)
	assert.NotNil(t, v4Client.ClustersApiInstance)
	assert.NotNil(t, v4Client.TasksApiInstance)
	assert.NotNil(t, v4Client.StorageContainerAPI)
	assert.NotNil(t, v4Client.DisksServiceApiInstance)
	assert.NotNil(t, v4Client.AlertsServiceApiInstance)
	assert.NotNil(t, v4Client.CategoriesApiInstance)
	assert.NotNil(t, v4Client.VolumeGroupsApiInstance)
	assert.NotNil(t, v4Client.VmProfilesApiInstance)

	// verify missing client scenario
	cred = prismgoclient.Credentials{
		URL:      "foo.com",
		Insecure: true,
		RequiredFields: map[string][]string{
			"prism_central": {"username", "password", "endpoint"},
		},
	}

	v4Client, err = NewV4Client(cred)
	assert.Nil(t, v4Client)
	assert.EqualError(t, err, "username, password and endpoint are required for basic auth")
}

func TestNewV4Client(t *testing.T) {
	// verifies positive client creation
	cred := prismgoclient.Credentials{
		URL:                "foo.com",
		APIKey:             "my-api-key",
		Port:               "",
		Endpoint:           "0.0.0.0",
		Insecure:           true,
		FoundationEndpoint: "10.0.0.0",
		FoundationPort:     "8000",
		RequiredFields:     nil,
	}
	v4Client, err := NewV4Client(cred)
	assert.NoError(t, err)
	assert.NotNil(t, v4Client)
	assert.NotNil(t, v4Client.VmApiInstance)
	assert.NotNil(t, v4Client.ImagesApiInstance)
	assert.NotNil(t, v4Client.SubnetsApiInstance)
	assert.NotNil(t, v4Client.SubnetIPReservationApi)
	assert.NotNil(t, v4Client.ClustersApiInstance)
	assert.NotNil(t, v4Client.TasksApiInstance)
	assert.NotNil(t, v4Client.StorageContainerAPI)
	assert.NotNil(t, v4Client.DisksServiceApiInstance)
	assert.NotNil(t, v4Client.AlertsServiceApiInstance)
	assert.NotNil(t, v4Client.CategoriesApiInstance)
	assert.NotNil(t, v4Client.VolumeGroupsApiInstance)
	assert.NotNil(t, v4Client.VmProfilesApiInstance)

	// verify missing client scenario
	cred = prismgoclient.Credentials{
		URL:      "foo.com",
		Insecure: true,
		RequiredFields: map[string][]string{
			"prism_central": {"username", "password", "endpoint"},
		},
		APIKey: "my-api-key",
	}

	v4Client, err = NewV4Client(cred)
	assert.Nil(t, v4Client)
	assert.EqualError(t, err, "endpoint is required for api key auth")
}

func TestInitAlertsServiceApiInstance(t *testing.T) {
	// happy path: instance is wired with endpoint, port and TLS settings
	cred := prismgoclient.Credentials{
		Username: "username",
		Password: "password",
		Endpoint: "0.0.0.0",
		Insecure: true,
	}
	v4Client := &Client{}
	err := initAlertsServiceApiInstance(v4Client, cred)
	assert.NoError(t, err)
	assert.NotNil(t, v4Client.AlertsServiceApiInstance)
	assert.Equal(t, "0.0.0.0", v4Client.AlertsServiceApiInstance.ApiClient.Host)
	port, convErr := strconv.Atoi(defaultEndpointPort)
	assert.NoError(t, convErr)
	assert.Equal(t, port, v4Client.AlertsServiceApiInstance.ApiClient.Port)
	assert.False(t, v4Client.AlertsServiceApiInstance.ApiClient.VerifySSL)

	// error path: invalid endpoint port makes getEndpointInfo fail
	badCred := prismgoclient.Credentials{
		Username: "username",
		Password: "password",
		Endpoint: "0.0.0.0:not-a-port",
		Insecure: true,
	}
	badClient := &Client{}
	err = initAlertsServiceApiInstance(badClient, badCred)
	assert.Error(t, err)
	assert.Nil(t, badClient.AlertsServiceApiInstance)
}

type fakeApiClientV4Test struct {
	headers      map[string]string
	caCerts      []byte
	caCertsError error
}

func (f *fakeApiClientV4Test) SetAdditionalCACertificates(certPEM []byte) error {
	if f.caCertsError != nil {
		return f.caCertsError
	}
	f.caCerts = certPEM
	return nil
}

func (f *fakeApiClientV4Test) AddDefaultHeader(key, value string) {
	if f.headers == nil {
		f.headers = make(map[string]string)
	}
	f.headers[key] = value
}

func Test_setAuthHeader_ServiceAccountModes_V4(t *testing.T) {
	tests := []struct {
		name               string
		creds              prismgoclient.Credentials
		expectedAPIKey     string
		expectAuth         bool
		expectedAuthPrefix string
	}{
		{
			name: "explicit API key",
			creds: prismgoclient.Credentials{
				APIKey: "explicit-api-key",
			},
			expectedAPIKey: "explicit-api-key",
			expectAuth:     false,
		},
		{
			name: "username is header key, password is API key",
			creds: prismgoclient.Credentials{
				Username: ntnxAPIKeyHeaderKey,
				Password: "password-as-api-key",
			},
			expectedAPIKey: "password-as-api-key",
			expectAuth:     false,
		},
		{
			name: "basic auth fallback",
			creds: prismgoclient.Credentials{
				Username: "user",
				Password: "pass",
			},
			expectedAPIKey:     "",
			expectAuth:         true,
			expectedAuthPrefix: "Basic ",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ac := &fakeApiClientV4Test{}
			setAuthHeader(ac, tt.creds)

			if tt.expectedAPIKey != "" {
				assert.Equal(t, tt.expectedAPIKey, ac.headers[ntnxAPIKeyHeaderKey])
			} else {
				_, hasAPIKey := ac.headers[ntnxAPIKeyHeaderKey]
				assert.False(t, hasAPIKey)
			}

			if tt.expectAuth {
				val, hasAuth := ac.headers[authorizationHeader]
				assert.True(t, hasAuth)
				if tt.expectedAuthPrefix != "" {
					assert.Contains(t, val, tt.expectedAuthPrefix)
				}
			} else {
				_, hasAuth := ac.headers[authorizationHeader]
				assert.False(t, hasAuth)
			}
		})
	}
}

func TestSetAdditionalTrustBundle(t *testing.T) {
	const pem = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"

	t.Run("bundle is forwarded to the SDK", func(t *testing.T) {
		ac := &fakeApiClientV4Test{}
		err := setAdditionalTrustBundle(ac, prismgoclient.Credentials{AdditionalTrustBundle: pem})
		assert.NoError(t, err)
		assert.Equal(t, []byte(pem), ac.caCerts)
	})

	t.Run("no bundle is a no-op", func(t *testing.T) {
		ac := &fakeApiClientV4Test{}
		err := setAdditionalTrustBundle(ac, prismgoclient.Credentials{})
		assert.NoError(t, err)
		assert.Nil(t, ac.caCerts)
	})

	t.Run("SDK error is propagated", func(t *testing.T) {
		ac := &fakeApiClientV4Test{caCertsError: errors.New("bad pem")}
		err := setAdditionalTrustBundle(ac, prismgoclient.Credentials{AdditionalTrustBundle: pem})
		assert.ErrorContains(t, err, "failed to set additional trust bundle")
	})

	// A trust bundle must not disable verification: it is supplied precisely so the endpoint's
	// certificate can be verified against it. Previously a bundle forced Insecure=true.
	t.Run("trust bundle keeps TLS verification enabled", func(t *testing.T) {
		v4Client := &Client{}
		err := initAlertsServiceApiInstance(v4Client, prismgoclient.Credentials{
			Username:              "username",
			Password:              "password",
			Endpoint:              "0.0.0.0",
			AdditionalTrustBundle: pem,
		})
		assert.NoError(t, err)
		assert.True(t, v4Client.AlertsServiceApiInstance.ApiClient.VerifySSL)
	})
}
