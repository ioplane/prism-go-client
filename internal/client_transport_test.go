package internal

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
)

func TestNewClientTransportIsolation(t *testing.T) {
	// Replace only for this sequential test, before any concurrent client creation.
	// A private fixture also prevents a failing regression from contaminating other tests.
	previous := http.DefaultTransport
	base := previous.(*http.Transport).Clone()
	tlsConfig := &tls.Config{ServerName: "default.example"}
	base.TLSClientConfig = tlsConfig
	defaultProxy, _ := url.Parse("http://default.example:3128")
	base.Proxy = http.ProxyURL(defaultProxy)
	http.DefaultTransport = base
	t.Cleanup(func() { http.DefaultTransport = previous })

	const count = 24
	clients := make([]*Client, count)
	errors := make([]error, count)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := range clients {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			clients[i], errors[i] = NewClient(
				WithCredentials(&prismgoclient.Credentials{
					URL: "https://prism.example", Insecure: i%2 == 0,
					ProxyURL: fmt.Sprintf("http://proxy-%d.example:3128", i),
				}), WithUserAgent("transport-test"), WithAbsolutePath("api/nutanix/v3"))
		}(i)
	}
	close(start)
	workers.Wait()

	request, _ := http.NewRequest(http.MethodGet, "https://prism.example", nil)
	seen := make(map[*http.Transport]bool)
	for i, client := range clients {
		if errors[i] != nil {
			t.Fatalf("client %d: %v", i, errors[i])
		}
		transport := client.httpClient.Transport.(*http.Transport)
		if transport == base || seen[transport] {
			t.Errorf("client %d shares its transport", i)
		}
		seen[transport] = true
		if transport.TLSClientConfig == tlsConfig {
			t.Errorf("client %d shares default TLS config", i)
		}
		if transport.TLSClientConfig.InsecureSkipVerify != (i%2 == 0) {
			t.Errorf("client %d has another client's TLS verification setting", i)
		}
		proxy, err := transport.Proxy(request)
		if err != nil || proxy == nil || proxy.Host != fmt.Sprintf("proxy-%d.example:3128", i) {
			t.Errorf("client %d has another client's proxy: %v, %v", i, proxy, err)
		}
		if transport.TLSClientConfig.RootCAs != client.certpool {
			t.Errorf("client %d does not use its own certificate pool", i)
		}
	}
	proxy, err := base.Proxy(request)
	if err != nil || proxy.String() != defaultProxy.String() || base.TLSClientConfig != tlsConfig || tlsConfig.InsecureSkipVerify || tlsConfig.RootCAs != nil {
		t.Error("default transport was mutated by client creation")
	}
	if len(clients) > 1 && clients[0].certpool == clients[1].certpool {
		t.Error("clients share certificate pools")
	}
}

type unsupportedDefaultTransport struct{}

func (unsupportedDefaultTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected HTTP request")
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
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(client.CloseIdleConnections)
			transport := client.ownedTransport
			if client.httpClient.Transport != transport || transport == http.DefaultTransport {
				t.Fatal("nil transport option discarded the private transport")
			}
			if !transport.TLSClientConfig.InsecureSkipVerify || transport.TLSClientConfig.RootCAs != client.certpool {
				t.Fatal("nil transport option discarded the client TLS configuration")
			}
			proxy, err := transport.Proxy(nil)
			if err != nil || proxy == nil || proxy.String() != credentials.ProxyURL {
				t.Fatalf("nil transport option discarded the client proxy: %v", err)
			}
		})
	}
}
