package internal

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func concurrentTransportClients() ([]*Client, []error) {
	const count = 24

	clients := make([]*Client, count)
	clientErrors := make([]error, count)
	start := make(chan struct{})

	var workers sync.WaitGroup
	for index := range clients {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()

			<-start

			clients[index], clientErrors[index] = NewClient(
				WithCredentials(&prismgoclient.Credentials{
					URL: "https://prism.example", Insecure: index%2 == 0,
					ProxyURL: fmt.Sprintf("http://proxy-%d.example:3128", index),
				}), WithUserAgent("transport-test"), WithAbsolutePath("api/nutanix/v3"))
		}(index)
	}

	close(start)
	workers.Wait()

	return clients, clientErrors
}

func TestNewClientTransportIsolation(t *testing.T) {
	// Replace only for this sequential test, before any concurrent client creation.
	// A private fixture also prevents a failing regression from contaminating other tests.
	previous := http.DefaultTransport
	original, ok := previous.(*http.Transport)
	require.True(t, ok)

	base := original.Clone()
	tlsConfig := &tls.Config{ServerName: "default.example"}
	base.TLSClientConfig = tlsConfig
	defaultProxy, err := url.Parse("http://default.example:3128")
	require.NoError(t, err)

	base.Proxy = http.ProxyURL(defaultProxy)
	http.DefaultTransport = base

	t.Cleanup(func() { http.DefaultTransport = previous })

	clients, clientErrors := concurrentTransportClients()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://prism.example", http.NoBody)
	require.NoError(t, err)

	seen := make(map[*http.Transport]bool)

	for index, client := range clients {
		require.NoError(t, clientErrors[index], "client %d", index)

		transport, ok := client.httpClient.Transport.(*http.Transport)
		require.True(t, ok, "client %d transport type", index)
		assert.NotSame(t, base, transport, "client %d shares default transport", index)
		assert.False(t, seen[transport], "client %d shares transport", index)
		seen[transport] = true
		assert.NotSame(t, tlsConfig, transport.TLSClientConfig, "client %d shares default TLS config", index)
		assert.Equal(t, index%2 == 0, transport.TLSClientConfig.InsecureSkipVerify, "client %d TLS verification", index)
		proxy, proxyErr := transport.Proxy(request)
		require.NoError(t, proxyErr)
		require.NotNil(t, proxy, "client %d proxy", index)
		assert.Equal(t, fmt.Sprintf("proxy-%d.example:3128", index), proxy.Host, "client %d proxy", index)
		assert.Same(t, client.certpool, transport.TLSClientConfig.RootCAs, "client %d certificate pool", index)
	}

	proxy, err := base.Proxy(request)
	require.NoError(t, err)
	assert.Equal(t, defaultProxy.String(), proxy.String(), "default proxy")
	assert.Same(t, tlsConfig, base.TLSClientConfig, "default TLS config")
	assert.False(t, tlsConfig.InsecureSkipVerify, "default TLS verification")
	assert.Nil(t, tlsConfig.RootCAs, "default certificate pool")
	assert.NotSame(t, clients[0].certpool, clients[1].certpool, "clients share certificate pools")
}

type unsupportedDefaultTransport struct{}

func (unsupportedDefaultTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected HTTP request")
}

func TestNewClientUnsupportedDefaultTransport(t *testing.T) {
	previous := http.DefaultTransport

	t.Cleanup(func() { http.DefaultTransport = previous })

	for _, transport := range []http.RoundTripper{unsupportedDefaultTransport{}, (*http.Transport)(nil), nil} {
		t.Run(fmt.Sprintf("%T", transport), func(t *testing.T) {
			http.DefaultTransport = transport

			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("NewClient panicked instead of returning an error: %v", recovered)
				}
			}()

			client, err := NewClient()
			if err == nil || client != nil {
				t.Errorf("unsupported default transport: client=%v, error=%v", client, err)
			}
		})
	}
}

func TestNewClientNilRoundTripper(t *testing.T) {
	credentials := &prismgoclient.Credentials{URL: "https://fixture.invalid", Insecure: true, ProxyURL: "http://proxy.invalid:3128"}

	useDefault := WithRoundTripper(nil)
	for name, options := range map[string][]ClientOption{
		"before credentials": {useDefault, WithCredentials(credentials)},
		"after credentials":  {WithCredentials(credentials), useDefault},
	} {
		t.Run(name, func(t *testing.T) {
			client, err := NewClient(append(options, WithUserAgent("fixture"), WithAbsolutePath("fixture"))...)
			require.NoError(t, err)
			t.Cleanup(client.CloseIdleConnections)
			transport := client.ownedTransport
			require.Same(t, transport, client.httpClient.Transport, "nil transport option discarded the private transport")
			require.NotSame(t, http.DefaultTransport, transport, "nil transport option shares the default transport")
			require.True(t, transport.TLSClientConfig.InsecureSkipVerify, "nil transport option discarded TLS verification setting")
			require.Same(t, client.certpool, transport.TLSClientConfig.RootCAs, "nil transport option discarded the client certificate pool")
			proxy, err := transport.Proxy(nil)
			require.NoError(t, err)
			require.NotNil(t, proxy, "nil transport option discarded the client proxy")
			require.Equal(t, credentials.ProxyURL, proxy.String(), "nil transport option discarded the client proxy")
		})
	}
}
