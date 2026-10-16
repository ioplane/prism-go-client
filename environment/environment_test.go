package environment

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	. "github.com/onsi/gomega"
)

type testProvider struct {
	// single management endpoint
	mgmtEndpoint *types.ManagementEndpoint
	// variables as JSON doc
	vars  map[string]interface{}
	err   error
	calls int
}

func (prov *testProvider) GetManagementEndpoint(
	topology types.Topology,
) (*types.ManagementEndpoint, error) {
	prov.calls++
	return prov.mgmtEndpoint, prov.err
}

func (prov *testProvider) Get(topology types.Topology, key string) (
	interface{}, error,
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
	g := NewWithT(t)
	me := &types.ManagementEndpoint{
		Address: &url.URL{Scheme: "https", Host: "pc"},
		ApiCredentials: types.ApiCredentials{
			Username: "k8s",
			Password: "k8s/4u",
		},
	}
	env := NewEnvironment(
		&testProvider{
			mgmtEndpoint: me,
			vars: map[string]interface{}{
				"foo": true,
			},
		},
		&testProvider{
			vars: map[string]interface{}{
				"bar": false,
			},
		},
	)

	val, err := env.Get(nil, "bar")
	g.Expect(err).To(BeNil())
	g.Expect(val).To(BeFalse())
	val, err = env.Get(nil, "foo")
	g.Expect(err).To(BeNil())
	g.Expect(val).To(BeTrue())
	_, err = env.Get(nil, "none")
	g.Expect(err).To(BeIdenticalTo(types.ErrNotFound))

	g.Expect(env.GetManagementEndpoint(nil)).To(BeIdenticalTo(me))
}

func TestEnvironmentProviderErrors(t *testing.T) {
	hardError := errors.New("provider unavailable")
	for _, test := range []struct {
		name         string
		firstError   error
		secondError  error
		wantError    error
		wantFallback bool
	}{
		{name: "wrapped absence", firstError: fmt.Errorf("provider: %w", types.ErrNotFound), wantFallback: true},
		{name: "hard error", firstError: hardError, wantError: hardError},
		{name: "nil success"},
		{
			name:         "all absent",
			firstError:   fmt.Errorf("provider: %w", types.ErrNotFound),
			secondError:  types.ErrNotFound,
			wantError:    types.ErrNotFound,
			wantFallback: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			endpoint := &types.ManagementEndpoint{}
			first := &testProvider{vars: map[string]interface{}{"fixture": nil}, err: test.firstError}
			second := &testProvider{mgmtEndpoint: endpoint, vars: map[string]interface{}{"fixture": true}, err: test.secondError}
			environment := NewEnvironment(first, second)
			value, valueError := environment.Get(nil, "fixture")
			managementEndpoint, endpointError := environment.GetManagementEndpoint(nil)
			if test.wantError == nil {
				g.Expect(valueError).NotTo(HaveOccurred())
				g.Expect(endpointError).NotTo(HaveOccurred())
			} else {
				g.Expect(valueError).To(BeIdenticalTo(test.wantError))
				g.Expect(endpointError).To(BeIdenticalTo(test.wantError))
			}
			if test.wantFallback && test.wantError == nil {
				g.Expect(value).To(BeTrue())
				g.Expect(managementEndpoint).To(BeIdenticalTo(endpoint))
			} else {
				g.Expect(value).To(BeNil())
				g.Expect(managementEndpoint).To(BeNil())
			}
			g.Expect(first.calls).To(Equal(2))
			if test.wantFallback {
				g.Expect(second.calls).To(Equal(2))
			} else {
				g.Expect(second.calls).To(BeZero())
			}
		})
	}
}
