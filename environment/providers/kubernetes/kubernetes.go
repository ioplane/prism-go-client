// Package kubernetes reads environment settings from informer caches.
package kubernetes

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	coreinformers "k8s.io/client-go/informers/core/v1"
)

type provider struct {
	secretInformer coreinformers.SecretInformer
	cmInformer     coreinformers.ConfigMapInformer
	prismEndpoint  credentials.NutanixPrismEndpoint
}

const (
	certBundleKey = "ca.crt"
)

type endpointParseError struct {
	cause error
}

func (*endpointParseError) Error() string { return "error parsing Kubernetes environment endpoint" }

func (err *endpointParseError) Unwrap() error { return err.cause }

// GetManagementEndpoint retrieves management endpoint.
func (prov *provider) GetManagementEndpoint(
	topology types.Topology,
) (*types.ManagementEndpoint, error) {
	creds, err := prov.getCredentials(topology)
	if err != nil {
		return nil, err
	}

	addr, err := url.Parse("https://" + prov.prismEndpoint.Address + ":" + strconv.Itoa(int(prov.prismEndpoint.Port)))
	if err != nil {
		return nil, &endpointParseError{cause: err}
	}

	trustBundle, err := prov.getAdditionalTrustBundle()
	if err != nil {
		return nil, err
	}

	return &types.ManagementEndpoint{
		Address:               addr,
		Insecure:              prov.prismEndpoint.Insecure,
		AdditionalTrustBundle: trustBundle,
		ApiCredentials:        *creds,
	}, nil
}

// Get retrieves settings applicable to the environment like project
// or category to which resources created on behalf of Kubernetes cluster are
// assigned to.
// These settings might have to be explicitly propagated to resources.
// Return whether lookup was successful to distinguish from nil as value.
func (*provider) Get(_ types.Topology, _ string) (
	any, error,
) {
	return nil, types.ErrNotFound
}

func (prov *provider) getAdditionalTrustBundle() (string, error) {
	if prov.prismEndpoint.AdditionalTrustBundle == nil {
		return "", nil
	}

	if prov.prismEndpoint.AdditionalTrustBundle.Kind == credentials.NutanixTrustBundleKindString {
		return prov.prismEndpoint.AdditionalTrustBundle.Data, nil
	}

	trustBundleRef := prov.prismEndpoint.AdditionalTrustBundle

	configMap, err := prov.cmInformer.Lister().ConfigMaps(trustBundleRef.Namespace).Get(trustBundleRef.Name)
	if err != nil {
		return "", err
	}

	if cert, ok := configMap.Data[certBundleKey]; ok {
		return cert, nil
	}

	if b64Cert, ok := configMap.BinaryData[certBundleKey]; ok {
		return string(b64Cert), nil
	}

	return "", nil
}

func (prov *provider) getCredentials(_ types.Topology) (*types.ApiCredentials, error) {
	ref := prov.prismEndpoint.CredentialRef
	if ref == nil {
		return nil, errors.New("credential reference must not be nil")
	}

	secret, err := prov.secretInformer.Lister().Secrets(ref.Namespace).Get(ref.Name)
	if err != nil {
		return nil, err
	}

	// Parse credentials in secret
	credsData, ok := secret.Data[credentials.KeyName]
	if !ok {
		return nil, fmt.Errorf("no %q data found in secret %s/%s",
			credentials.KeyName, ref.Namespace, ref.Name)
	}

	return credentials.ParseCredentials(credsData)
}

// NewProvider constructs Kubernetes Environment provider taking
// Prism endpoint and a secretes informer as input.
// It's assumed secrets informer is already running and its cache has been synced.
func NewProvider(
	prismEndpoint credentials.NutanixPrismEndpoint,
	secretInformer coreinformers.SecretInformer,
	cmInformer coreinformers.ConfigMapInformer,
) types.Provider {
	return &provider{
		prismEndpoint:  prismEndpoint,
		secretInformer: secretInformer,
		cmInformer:     cmInformer,
	}
}
