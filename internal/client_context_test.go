package internal_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-logr/logr"
	prism "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/internal"
	"github.com/nutanix-cloud-native/prism-go-client/konnector"
	"github.com/nutanix-cloud-native/prism-go-client/v3"
	"github.com/nutanix-cloud-native/prism-go-client/v4"
	"github.com/stretchr/testify/require"
)

func TestConstructorContextCancellation(t *testing.T) {
	constructors := map[string]func(context.Context, prism.Credentials) (any, error){
		"shared": func(ctx context.Context, c prism.Credentials) (any, error) {
			return internal.NewClientWithContext(ctx, internal.WithCredentials(&c), internal.WithUserAgent("fixture"), internal.WithAbsolutePath("api/nutanix/v3"))
		},
		"v3": func(ctx context.Context, c prism.Credentials) (any, error) { return v3.NewV3ClientWithContext(ctx, c) },
		"konnector": func(ctx context.Context, c prism.Credentials) (any, error) {
			return konnector.NewKonnectorAPIClientWithContext(ctx, c)
		},
	}
	for name, construct := range constructors {
		t.Run(name, func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			credentials := prism.Credentials{URL: server.URL, Endpoint: server.URL, APIKey: "fixture", SessionAuth: true}
			type result struct {
				client any
				err    error
			}
			done := make(chan result, 1)
			go func() { client, err := construct(ctx, credentials); done <- result{client, err} }()
			<-started
			cancel()
			got := <-done
			require.Nil(t, got.client)
			require.ErrorIs(t, got.err, context.Canceled)
			client, err := construct(ctx, credentials)
			require.Nil(t, client)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

type contextTransport struct{ closed atomic.Int32 }

func (*contextTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.Canceled
}
func (t *contextTransport) CloseIdleConnections() { t.closed.Add(1) }

func TestClientCallerOwnedTransport(t *testing.T) {
	for name, caller := range map[string]*contextTransport{"non-nil": {}, "nil receiver": nil} {
		t.Run(name, func(t *testing.T) {
			credentials := prism.Credentials{URL: "https://fixture.invalid", APIKey: "fixture"}
			client, err := internal.NewClient(internal.WithCredentials(&credentials), internal.WithUserAgent("fixture"), internal.WithAbsolutePath("api/nutanix/v3"), internal.WithRoundTripper(caller))
			require.NoError(t, err)
			request, err := client.NewRequest(http.MethodGet, "/fixture", nil)
			require.NoError(t, err)
			require.ErrorIs(t, client.Do(context.Background(), request, nil), context.Canceled)
			client.CloseIdleConnections()
			if caller != nil {
				assert.Zero(t, caller.closed.Load(), "caller transport remains caller-owned")
			}
		})
	}
}

func TestConstructorNilInputs(t *testing.T) {
	credentials := prism.Credentials{URL: "https://fixture.invalid", Endpoint: "fixture.invalid", APIKey: "fixture"}
	for name, construct := range map[string]func() (any, error){
		"credentials":   func() (any, error) { return internal.NewClient(internal.WithCredentials(nil)) },
		"shared option": func() (any, error) { return internal.NewClient(nil) },
		"typed nil HTTP transport": func() (any, error) {
			return internal.NewClient(internal.WithCredentials(&credentials), internal.WithUserAgent("fixture"),
				internal.WithAbsolutePath("fixture"), internal.WithRoundTripper((*http.Transport)(nil)))
		},
		"logger": func() (any, error) {
			return internal.NewClient(internal.WithLogger(nil), internal.WithCredentials(&credentials),
				internal.WithUserAgent("fixture"), internal.WithAbsolutePath("fixture"))
		},
		"v3 option":        func() (any, error) { return v3.NewV3Client(credentials, nil) },
		"v4 option":        func() (any, error) { return v4.NewV4Client(credentials, nil) },
		"konnector option": func() (any, error) { return konnector.NewKonnectorAPIClient(credentials, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			require.NotPanics(t, func() {
				client, err := construct()
				require.Nil(t, client)
				require.Error(t, err)
			})
		})
	}
}

func TestConstructorURLDiagnostics(t *testing.T) {
	const marker = "synthetic-url-secret-marker"
	for _, test := range []struct {
		name    string
		proxy   string
		baseURL string
		invalid bool
	}{
		{name: "valid proxy", proxy: "http://fixture:" + marker + "@proxy.invalid:3128", baseURL: "https://fixture.invalid/"},
		{name: "malformed proxy", proxy: "http://fixture:" + marker + "@%41.invalid", baseURL: "https://fixture.invalid/", invalid: true},
		{name: "valid API URL", baseURL: "https://fixture:" + marker + "@fixture.invalid/path?token=" + marker},
		{name: "malformed API URL", baseURL: "https://fixture:" + marker + "@%41.invalid/path?token=" + marker, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := logr.FromSlogHandler(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			credentials := prism.Credentials{URL: test.baseURL, ProxyURL: test.proxy}
			client, err := internal.NewClient(internal.WithLogger(&logger), internal.WithCredentials(&credentials),
				internal.WithUserAgent("fixture"), internal.WithAbsolutePath("fixture"))
			if test.invalid {
				require.Nil(t, client)
				require.Error(t, err)
				require.NotContains(t, err.Error(), marker)
				var cause *url.Error
				require.ErrorAs(t, err, &cause)
				require.Contains(t, cause.URL, marker)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.baseURL, client.BaseURL.String())
				client.CloseIdleConnections()
			}
			require.NotContains(t, logs.String(), marker)
		})
	}
}
