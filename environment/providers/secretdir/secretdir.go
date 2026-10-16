// Package secretdir reads endpoint settings and credentials from mounted secret files.
package secretdir

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
)

const (
	csiCredentialFieldCount = 4
	csiPasswordIndex        = 3
	secretKeyName           = "key"
	secretKeyCertName       = "cert"
	secretKeyEndpoint       = "endpoint"
	secretKeyInsecure       = "insecure"
	secretKeyCredentials    = credentials.KeyName
	envCSISecretDir         = "CSI_SECRET_DIR"
	envSecretDir            = "SECRET_DIR"
)

// ErrMissingValue identifies a malformed legacy CSI credential value.
var ErrMissingValue = errors.New("missing information in secret value " +
	"'<prism-ip>:<prism-port>:<user>:<password>'")

type provider struct{}

// GetManagementEndpoint returns PC credentials independent of HCI cluster.
func (*provider) GetManagementEndpoint(
	_ types.Topology,
) (*types.ManagementEndpoint, error) {
	path := os.Getenv(envSecretDir)
	if path == "" {
		// fall back to CSI env var
		path = os.Getenv(envCSISecretDir)
		if path == "" {
			return nil, fmt.Errorf("neither %s nor %s environment variable set",
				envSecretDir, envCSISecretDir)
		}
	}

	return getMgmtEndpointFromSecretDir(path)
}

// Get doesn't return any settings from CSI secret.
func (*provider) Get(_ types.Topology, _ string) (
	any, error,
) {
	return nil, types.ErrNotFound
}

// NewProvider returns a provider backed by a mounted secret directory.
func NewProvider() types.Provider {
	return &provider{}
}

// read parameters return "" for non-existing keys.
func readParam(path, key string) (string, error) {
	data, readErr := os.ReadFile(filepath.Join(path, key))
	if readErr == nil {
		return string(data), nil
	}

	if os.IsNotExist(readErr) {
		return "", nil
	}

	return "", readErr
}

func getAPIEndpoint(path string) (*url.URL, error) {
	endpoint, err := readParam(path, secretKeyEndpoint)
	if err != nil {
		return nil, err
	}

	if endpoint == "" {
		return nil, errors.New("no endpoint is provided in secret value")
	}

	return url.Parse("https://" + endpoint)
}

func getInsecure(path string) (bool, error) {
	val, err := readParam(path, secretKeyInsecure)
	if err != nil {
		return false, err
	}

	if val == "" {
		return false, nil
	}

	switch val {
	case "True", "true", "1", "on", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// mangementEndpointFromSecretParams parses management endpoint and credentials from
// parameters embedded as data in a k8s secret.
func getMgmtEndpointFromSecretDir(path string) (
	*types.ManagementEndpoint, error,
) {
	// Auto-detect new converged secret format
	credsData, err := readParam(path, secretKeyCredentials)
	if err != nil {
		return nil, err
	}

	if credsData != "" {
		apiCreds, parseErr := credentials.ParseCredentials([]byte(credsData))
		if parseErr != nil {
			return nil, parseErr
		}

		addr, endpointErr := getAPIEndpoint(path)
		if endpointErr != nil {
			return nil, endpointErr
		}

		insecure, insecureErr := getInsecure(path)
		if insecureErr != nil {
			return nil, insecureErr
		}

		return &types.ManagementEndpoint{
			Address:        addr,
			Insecure:       insecure,
			ApiCredentials: *apiCreds,
		}, nil
	}

	// Handle CSI format
	certString, err := readParam(path, secretKeyCertName)
	if err != nil {
		return nil, err
	}

	if certString != "" {
		// TLS key pair
		addr, endpointErr := getAPIEndpoint(path)
		if endpointErr != nil {
			return nil, endpointErr
		}

		return &types.ManagementEndpoint{
			KeyPair: certString,
			Address: addr,
		}, nil
	}
	// basic auth
	str, err := readParam(path, secretKeyName)
	if err != nil {
		return nil, err
	}

	creds := strings.SplitN(str, ":", csiCredentialFieldCount)
	if len(creds) != csiCredentialFieldCount {
		return nil, ErrMissingValue
	}

	addr, err := url.Parse("https://" + creds[0] + ":" + creds[1])
	if err != nil {
		return nil, err
	}

	return &types.ManagementEndpoint{
		Username: creds[2],
		Password: creds[csiPasswordIndex],
		Address:  addr,
	}, nil
}
