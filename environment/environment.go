// Package environment looks up cluster configuration through ordered providers.
package environment

import (
	"errors"

	"github.com/nutanix-cloud-native/prism-go-client/environment/providers/configmap"
	"github.com/nutanix-cloud-native/prism-go-client/environment/providers/local"
	"github.com/nutanix-cloud-native/prism-go-client/environment/providers/secretdir"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
)

// Env is current environment. Environment is a singleton as k8s cluster
// can't span multiple environment. Providers of environment hide behind
// this interface.
// Clients of this package are free to reconfigure environment.
var Env = NewEnvironment(
	local.NewProvider(),     // Give local provider highest priority.
	secretdir.NewProvider(), // Secret and config map env shouldn't intersect,
	configmap.NewProvider(), // hence order doesn't matter.
)

// GetManagementEndpoint is convenience function.
func GetManagementEndpoint(topology types.Topology) (
	*types.ManagementEndpoint, error,
) {
	return Env.GetManagementEndpoint(topology)
}

// Get is convenience function.
func Get(topology types.Topology, key string) (any, error) {
	return Env.Get(topology, key)
}

// env construct environment from various sources.
type env struct {
	providers []types.Provider
}

// GetManagementEndpoint implements Environment interface.
func (e *env) GetManagementEndpoint(topology types.Topology) (
	*types.ManagementEndpoint, error,
) {
	for _, provider := range e.providers {
		if me, err := provider.GetManagementEndpoint(topology); err == nil ||
			!errors.Is(err, types.ErrNotFound) {
			return me, err
		}
	}

	return nil, types.ErrNotFound
}

// Get implements Environment interface.
func (e *env) Get(topology types.Topology, key string) (any, error) {
	for _, provider := range e.providers {
		if val, err := provider.Get(topology, key); err == nil ||
			!errors.Is(err, types.ErrNotFound) {
			return val, err
		}
	}

	return nil, types.ErrNotFound
}

// NewEnvironment constructs new environment from given list of
// providers. The order of providers is honored during lookup.
// This function is meant to construct Env as environment is a
// singleton.
func NewEnvironment(p ...types.Provider) types.Environment {
	return &env{
		providers: p,
	}
}
