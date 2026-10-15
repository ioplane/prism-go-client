package internal_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	prism "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/internal"
	"github.com/nutanix-cloud-native/prism-go-client/konnector"
	"github.com/nutanix-cloud-native/prism-go-client/v3"
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
	caller := &contextTransport{}
	credentials := prism.Credentials{URL: "https://fixture.invalid", APIKey: "fixture"}
	client, err := internal.NewClient(internal.WithCredentials(&credentials), internal.WithUserAgent("fixture"), internal.WithAbsolutePath("api/nutanix/v3"), internal.WithRoundTripper(caller))
	require.NoError(t, err)
	client.CloseIdleConnections()
	assert.Zero(t, caller.closed.Load(), "caller transport remains caller-owned")
}
