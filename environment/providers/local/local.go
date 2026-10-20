// Package local reads management endpoints, credentials and settings from environment variables.
package local

import (
	"net/url"
	"os"
	"strings"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
)

const (
	endpointEnv    = "NUTANIX_ENDPOINT"
	portEnv        = "NUTANIX_PORT"
	userEnv        = "NUTANIX_USERNAME"
	passwordEnv    = "NUTANIX_PASSWORD"
	apiKeyEnv      = "NUTANIX_API_KEY"
	insecureEnv    = "NUTANIX_INSECURE"
	trustBundleEnv = "NUTANIX_ADDITIONAL_TRUST_BUNDLE"
	categoriesEnv  = "NUTANIX_CATEGORIES"
)

type provider struct{}

type endpointParseError struct {
	cause error
}

func (*endpointParseError) Error() string { return "error parsing local environment endpoint" }

func (err *endpointParseError) Unwrap() error { return err.cause }

func (*provider) GetManagementEndpoint(
	_ types.Topology,
) (*types.ManagementEndpoint, error) {
	endpoint := os.Getenv(endpointEnv)
	// No local environment defined
	if endpoint == "" {
		return nil, types.ErrNotFound
	}

	port := os.Getenv(portEnv)
	if port == "" {
		port = "9440"
	}

	address := endpoint + ":" + port
	if !strings.HasPrefix(address, "https://") {
		address = "https://" + address
	}

	addr, err := url.Parse(address)
	if err != nil {
		return nil, &endpointParseError{cause: err}
	}

	insecureTLS := os.Getenv(insecureEnv) == "true"
	trustBundle := os.Getenv(trustBundleEnv)

	credentials := &types.ApiCredentials{
		Username: os.Getenv(userEnv),
		Password: os.Getenv(passwordEnv),
		APIKey:   os.Getenv(apiKeyEnv),
		KeyPair:  "",
	}
	if err := credentials.Validate(); err != nil {
		return nil, err
	}

	return &types.ManagementEndpoint{
		Address:               addr,
		ApiCredentials:        *credentials,
		Insecure:              insecureTLS,
		AdditionalTrustBundle: trustBundle,
	}, nil
}

func (*provider) Get(_ types.Topology, key string) (
	any, error,
) {
	if key == types.CategoriesKey {
		return strings.Split(os.Getenv(categoriesEnv), ","), nil
	}

	return nil, types.ErrNotFound
}

// NewProvider returns a provider backed by local environment variables.
func NewProvider() types.Provider {
	return &provider{}
}
