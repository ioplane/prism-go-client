package internal_test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-logr/logr"
	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/internal"
	"github.com/nutanix-cloud-native/prism-go-client/konnector"
	"github.com/nutanix-cloud-native/prism-go-client/v3"
	"github.com/nutanix-cloud-native/prism-go-client/v4"
	"github.com/stretchr/testify/require"
)

const (
	testEndpoint = "https://fixture.invalid"
	testFixture  = "fixture"
)

func TestConstructorContextCancellation(t *testing.T) {
	constructors := map[string]func(context.Context, prismgoclient.Credentials) (any, error){
		"shared": func(ctx context.Context, c prismgoclient.Credentials) (any, error) {
			return internal.NewClientWithContext(ctx, internal.WithCredentials(&c),
				internal.WithUserAgent(testFixture), internal.WithAbsolutePath("api/nutanix/v3"))
		},
		"v3": func(ctx context.Context, c prismgoclient.Credentials) (any, error) {
			return v3.NewV3ClientWithContext(ctx, c)
		},
		"konnector": func(ctx context.Context, c prismgoclient.Credentials) (any, error) {
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

			credentials := prismgoclient.Credentials{URL: server.URL, Endpoint: server.URL, APIKey: testFixture, SessionAuth: true}

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

type contextTransport struct {
	closed atomic.Int32
	cause  error
}

func (transport *contextTransport) RoundTrip(*http.Request) (*http.Response, error) {
	if transport != nil && transport.cause != nil {
		return nil, transport.cause
	}

	return nil, context.Canceled
}
func (transport *contextTransport) CloseIdleConnections() { transport.closed.Add(1) }

func TestClientCallerOwnedTransport(t *testing.T) {
	for name, caller := range map[string]*contextTransport{"non-nil": {}, "nil receiver": nil} {
		t.Run(name, func(t *testing.T) {
			credentials := prismgoclient.Credentials{URL: testEndpoint, APIKey: testFixture}
			client, err := internal.NewClient(internal.WithCredentials(&credentials), internal.WithUserAgent(testFixture),
				internal.WithAbsolutePath("api/nutanix/v3"), internal.WithRoundTripper(caller))
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
	credentials := prismgoclient.Credentials{URL: testEndpoint, Endpoint: "fixture.invalid", APIKey: testFixture}

	for name, construct := range map[string]func() (any, error){
		"credentials":   func() (any, error) { return internal.NewClient(internal.WithCredentials(nil)) },
		"shared option": func() (any, error) { return internal.NewClient(nil) },
		"typed nil HTTP transport": func() (any, error) {
			return internal.NewClient(internal.WithCredentials(&credentials), internal.WithUserAgent(testFixture),
				internal.WithAbsolutePath(testFixture), internal.WithRoundTripper((*http.Transport)(nil)))
		},
		"logger": func() (any, error) {
			return internal.NewClient(internal.WithLogger(nil), internal.WithCredentials(&credentials),
				internal.WithUserAgent(testFixture), internal.WithAbsolutePath(testFixture))
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
			credentials := prismgoclient.Credentials{URL: test.baseURL, ProxyURL: test.proxy}

			client, err := internal.NewClient(internal.WithLogger(&logger), internal.WithCredentials(&credentials),
				internal.WithUserAgent(testFixture), internal.WithAbsolutePath(testFixture))
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

func TestRequestURLDiagnostics(t *testing.T) {
	const (
		marker       = "synthetic-request-url-marker"
		path         = "/bad%zz?token=" + marker
		absolutePath = "api/nutanix/v3"
	)

	client, err := internal.NewClient(
		internal.WithCredentials(&prismgoclient.Credentials{URL: testEndpoint}),
		internal.WithUserAgent(testFixture), internal.WithAbsolutePath(absolutePath))
	require.NoError(t, err)

	builders := []struct {
		name  string
		build func() (*http.Request, error)
	}{
		{"unauth", func() (*http.Request, error) { return client.NewUnAuthRequest(http.MethodGet, path, nil) }},
		{"auth", func() (*http.Request, error) { return client.NewRequest(http.MethodGet, path, nil) }},
		{"form", func() (*http.Request, error) { return client.NewUnAuthFormEncodedRequest(http.MethodPost, path, nil) }},
		{"upload", func() (*http.Request, error) { return client.NewUnAuthUploadRequest(http.MethodPut, path, nil) }},
		{"auth-upload", func() (*http.Request, error) { return client.NewUploadRequest(http.MethodPut, path, nil) }},
	}
	for _, builder := range builders {
		t.Run(builder.name, func(t *testing.T) {
			request, err := builder.build()
			require.Nil(t, request)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), marker)

			var cause *url.Error
			require.ErrorAs(t, err, &cause)
			assert.Equal(t, absolutePath+path, cause.URL)
			assert.Equal(t, "parse", cause.Op)
		})
	}
}

func TestTransportURLDiagnostics(t *testing.T) {
	const (
		marker = "synthetic-transport-url-marker"
		target = "https://fixture.invalid/request?token=" + marker
	)

	for _, test := range []struct {
		name     string
		filtered bool
		cause    error
		timeout  bool
	}{
		{"Do", false, context.Canceled, false},
		{"DoWithFilters", true, context.DeadlineExceeded, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := internal.NewClient(
				internal.WithCredentials(&prismgoclient.Credentials{URL: testEndpoint}),
				internal.WithRoundTripper(&contextTransport{cause: test.cause}),
				internal.WithUserAgent(testFixture), internal.WithAbsolutePath("api/nutanix/v3"))
			require.NoError(t, err)
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
			require.NoError(t, err)

			if test.filtered {
				err = client.DoWithFilters(t.Context(), request, nil, nil, nil)
			} else {
				err = client.Do(t.Context(), request, nil)
			}

			require.Error(t, err)
			assert.NotContains(t, err.Error(), marker)
			require.ErrorIs(t, err, test.cause)

			var cause *url.Error
			require.ErrorAs(t, err, &cause)
			assert.Equal(t, target, cause.URL)
			assert.Equal(t, "Get", cause.Op)

			networkError, ok := err.(net.Error)
			require.True(t, ok)
			assert.Equal(t, test.timeout, networkError.Timeout())
			assert.Equal(t, test.timeout, os.IsTimeout(err))
			temporary, ok := err.(interface{ Temporary() bool })
			require.True(t, ok)
			assert.Equal(t, test.timeout, temporary.Temporary())
		})
	}
}
