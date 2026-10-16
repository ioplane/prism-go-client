// Package configmap supplies a placeholder provider that reports missing configuration.
// Config map lookup is not implemented.
package configmap

import "github.com/nutanix-cloud-native/prism-go-client/environment/types"

type provider struct{}

func (*provider) GetManagementEndpoint(
	_ types.Topology,
) (*types.ManagementEndpoint, error) {
	return nil, types.ErrNotFound
}

func (*provider) Get(_ types.Topology, _ string) (
	any, error,
) {
	return nil, types.ErrNotFound
}

// NewProvider returns a provider that reports missing configuration.
func NewProvider() types.Provider {
	return &provider{}
}
