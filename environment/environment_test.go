package environment_test

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/nutanix-cloud-native/prism-go-client/environment"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	"github.com/onsi/gomega"
)

type testProvider struct {
	// single management endpoint
	mgmtEndpoint *types.ManagementEndpoint
	// variables as JSON doc
	vars  map[string]any
	err   error
	calls int
}

func (prov *testProvider) GetManagementEndpoint(
	_ types.Topology,
) (*types.ManagementEndpoint, error) {
	prov.calls++

	return prov.mgmtEndpoint, prov.err
}

func (prov *testProvider) Get(_ types.Topology, key string) (
	any, error,
) {
	prov.calls++
	if prov.err != nil {
		return nil, prov.err
	}

	if val, ok := prov.vars[key]; ok {
		return val, nil
	}

	return nil, types.ErrNotFound
}

func TestEnvironment(t *testing.T) {
	matcher := gomega.NewWithT(t)
	managementEndpoint := &types.ManagementEndpoint{
		Address:  &url.URL{Scheme: "https", Host: "pc"},
		Username: "k8s",
		Password: "k8s/4u",
	}
	env := environment.NewEnvironment(
		&testProvider{
			err:          nil,
			calls:        0,
			mgmtEndpoint: managementEndpoint,
			vars: map[string]any{
				"foo": true,
			},
		},
		&testProvider{
			mgmtEndpoint: nil,
			err:          nil,
			calls:        0,
			vars: map[string]any{
				"bar": false,
			},
		},
	)

	val, err := env.Get(nil, "bar")
	matcher.Expect(err).ToNot(gomega.HaveOccurred())
	matcher.Expect(val).To(gomega.BeFalse())
	val, err = env.Get(nil, "foo")
	matcher.Expect(err).ToNot(gomega.HaveOccurred())
	matcher.Expect(val).To(gomega.BeTrue())

	_, err = env.Get(nil, "none")
	matcher.Expect(err).To(gomega.BeIdenticalTo(types.ErrNotFound))

	matcher.Expect(env.GetManagementEndpoint(nil)).To(gomega.BeIdenticalTo(managementEndpoint))
}

type providerErrorCase struct {
	name         string
	firstError   error
	secondError  error
	wantError    error
	wantFallback bool
}

func providerErrorCases(hardError error) []providerErrorCase {
	return []providerErrorCase{
		{
			secondError:  nil,
			wantError:    nil,
			name:         "wrapped absence",
			firstError:   fmt.Errorf("provider: %w", types.ErrNotFound),
			wantFallback: true,
		},
		{
			secondError:  nil,
			wantFallback: false,
			name:         "hard error",
			firstError:   hardError,
			wantError:    hardError,
		},
		{
			firstError:   nil,
			secondError:  nil,
			wantError:    nil,
			wantFallback: false,
			name:         "nil success",
		},
		{
			name:         "all absent",
			firstError:   fmt.Errorf("provider: %w", types.ErrNotFound),
			secondError:  types.ErrNotFound,
			wantError:    types.ErrNotFound,
			wantFallback: true,
		},
	}
}

func TestEnvironmentProviderErrors(t *testing.T) {
	hardError := errors.New("provider unavailable")
	for _, test := range providerErrorCases(hardError) {
		t.Run(test.name, func(t *testing.T) {
			matcher := gomega.NewWithT(t)
			endpoint := &types.ManagementEndpoint{}
			first := &testProvider{
				mgmtEndpoint: nil,
				calls:        0,
				vars:         map[string]any{"fixture": nil},
				err:          test.firstError,
			}
			second := &testProvider{
				calls:        0,
				mgmtEndpoint: endpoint,
				vars:         map[string]any{"fixture": true},
				err:          test.secondError,
			}
			env := environment.NewEnvironment(first, second)
			value, valueError := env.Get(nil, "fixture")
			managementEndpoint, endpointError := env.GetManagementEndpoint(nil)

			if test.wantError == nil {
				matcher.Expect(valueError).NotTo(gomega.HaveOccurred())
				matcher.Expect(endpointError).NotTo(gomega.HaveOccurred())
			} else {
				matcher.Expect(valueError).To(gomega.BeIdenticalTo(test.wantError))
				matcher.Expect(endpointError).To(gomega.BeIdenticalTo(test.wantError))
			}

			if test.wantFallback && test.wantError == nil {
				matcher.Expect(value).To(gomega.BeTrue())
				matcher.Expect(managementEndpoint).To(gomega.BeIdenticalTo(endpoint))
			} else {
				matcher.Expect(value).To(gomega.BeNil())
				matcher.Expect(managementEndpoint).To(gomega.BeNil())
			}

			matcher.Expect(first.calls).To(gomega.Equal(2))

			if test.wantFallback {
				matcher.Expect(second.calls).To(gomega.Equal(2))
			} else {
				matcher.Expect(second.calls).To(gomega.BeZero())
			}
		})
	}
}
