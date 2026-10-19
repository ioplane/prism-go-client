package v3

import (
	"context"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheConcurrentConstruction(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const workers = 8

		cache := NewClientCache()
		params := &cachedClientParams{name: "fixture", mgmtEndpoint: types.ManagementEndpoint{
			Address: &url.URL{Host: "fixture.invalid"}, ApiCredentials: types.ApiCredentials{APIKey: "fixture"},
		}}
		release := make(chan struct{})

		var constructors atomic.Int32

		option := func(*Client) error {
			constructors.Add(1)
			<-release

			return nil
		}
		clients := make(chan *Client, workers)

		failures := make(chan error, workers)
		for range workers {
			go func() { client, err := cache.GetOrCreate(params, option); clients <- client; failures <- err }()
		}

		synctest.Wait()
		assert.Equal(t, int32(1), constructors.Load(), "one constructor per cache key")
		close(release)

		first := <-clients
		require.NotNil(t, first)

		for range workers - 1 {
			assert.Same(t, first, <-clients)
		}

		for range workers {
			require.NoError(t, <-failures)
		}
	})
}

func cacheFixtureParams() *cachedClientParams {
	return &cachedClientParams{name: "fixture", mgmtEndpoint: types.ManagementEndpoint{
		Address: &url.URL{Host: "fixture.invalid"}, ApiCredentials: types.ApiCredentials{APIKey: "fixture"},
	}}
}

func TestCacheCanceledWaiterAndIndependentKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := NewClientCache()
		params := cacheFixtureParams()
		release := make(chan struct{})
		owner := make(chan error, 1)
		oldFailure := errors.New("old credential failure")

		go func() {
			_, err := cache.GetOrCreate(params, func(*Client) error {
				<-release

				return oldFailure
			})
			owner <- err
		}()

		synctest.Wait()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		waiter := make(chan error, 1)

		go func() { _, err := cache.GetOrCreateWithContext(ctx, params); waiter <- err }()

		synctest.Wait()

		rotated := cacheFixtureParams()
		rotated.mgmtEndpoint.APIKey = "rotated"
		replacement := make(chan error, 1)

		go func() { _, err := cache.GetOrCreate(rotated); replacement <- err }()

		synctest.Wait()

		other := cacheFixtureParams()
		other.name = "independent"
		client, err := cache.GetOrCreate(other)
		require.NoError(t, err)
		require.NotNil(t, client)
		cancel()
		require.ErrorIs(t, <-waiter, context.Canceled)
		close(release)
		require.ErrorIs(t, <-owner, oldFailure)
		require.NoError(t, <-replacement)

		client, err = cache.GetOrCreateWithContext(ctx, params)
		require.Nil(t, client)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestCacheDeleteBlocksOldPublication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := NewClientCache()
		params := cacheFixtureParams()
		release := make(chan struct{})

		type result struct {
			client *Client
			err    error
		}

		owner := make(chan result, 1)

		go func() {
			client, err := cache.GetOrCreate(params, func(*Client) error {
				<-release

				return nil
			})
			owner <- result{client, err}
		}()

		synctest.Wait()
		cache.Delete(params)

		rotated := cacheFixtureParams()
		rotated.mgmtEndpoint.APIKey = "rotated"
		published, err := cache.GetOrCreate(rotated)
		require.NoError(t, err)
		close(release)

		old := <-owner
		require.Nil(t, old.client)
		require.ErrorIs(t, old.err, types.ErrorClientNotFound)
		actual, err := cache.GetOrCreate(rotated)
		require.NoError(t, err)
		assert.Same(t, published, actual)
	})
}

func TestCacheSharedFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const workers = 8

		cache := NewClientCache()
		params := cacheFixtureParams()
		release := make(chan struct{})
		failures := make(chan error, workers)

		var constructors atomic.Int32

		want := errors.New("constructor rejected")
		option := func(*Client) error {
			constructors.Add(1)
			<-release

			return want
		}

		for range workers {
			go func() { _, err := cache.GetOrCreate(params, option); failures <- err }()
		}

		synctest.Wait()
		assert.Equal(t, int32(1), constructors.Load())
		close(release)

		for range workers {
			require.ErrorIs(t, <-failures, want)
		}

		client, err := cache.GetOrCreate(params)
		require.NoError(t, err)
		require.NotNil(t, client)
	})
}

func TestCacheRotationAndFirstOptions(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()

	bundle := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	cache := NewClientCache()
	params := cacheFixtureParams()
	first, err := cache.GetOrCreate(params)
	require.NoError(t, err)

	var constructors int

	option := func(*Client) error {
		constructors++

		return nil
	}
	hit, err := cache.GetOrCreate(params, option)
	require.NoError(t, err)
	assert.Same(t, first, hit)
	assert.Zero(t, constructors, "cache hit ignores later functional options")

	rotated := cacheFixtureParams()
	rotated.mgmtEndpoint.APIKey = "rotated"
	second, err := cache.GetOrCreate(rotated, option)
	require.NoError(t, err)
	assert.NotSame(t, first, second)
	assert.Equal(t, 1, constructors)

	_, err = cache.GetOrCreate(params, func(*Client) error { return errors.New("invalid replacement") })
	require.Error(t, err)
	kept, err := cache.GetOrCreate(rotated)
	require.NoError(t, err)
	assert.Same(t, second, kept, "failed replacement preserves published client")

	for _, change := range []func(*types.ManagementEndpoint){
		func(ep *types.ManagementEndpoint) { ep.APIKey = ""; ep.Username = "fixture"; ep.Password = "first" },
		func(ep *types.ManagementEndpoint) { ep.Password = "rotated" },
		func(ep *types.ManagementEndpoint) { ep.Insecure = true },
		func(ep *types.ManagementEndpoint) { ep.AdditionalTrustBundle = bundle },
	} {
		previous := kept

		change(&rotated.mgmtEndpoint)
		kept, err = cache.GetOrCreate(rotated, option)
		require.NoError(t, err)
		assert.NotSame(t, previous, kept, "TLS or trust rotation replaces cached client")
	}

	cache.Delete(rotated)

	options := make([]types.ClientOption[Client], 1, 2)
	options[0] = option
	_, err = cache.GetOrCreate(rotated, options...)
	require.NoError(t, err)
	assert.Nil(t, options[:2][1], "caller option backing array remains unchanged")
}

func TestCacheSessionAuthenticationAndOwnedPool(t *testing.T) {
	var authCalls, connections atomic.Int32

	closed := make(chan struct{}, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCalls.Add(1)
		http.SetCookie(w, &http.Cookie{Name: "session-id", Value: "fixture"})
		w.WriteHeader(http.StatusNoContent)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			connections.Add(1)
		case http.StateClosed:
			closed <- struct{}{}
		}
	}

	server.StartTLS()
	defer server.Close()

	cache := NewClientCache(WithSessionAuth(true))
	params := cacheFixtureParams()
	params.mgmtEndpoint.Address = &url.URL{Host: server.Listener.Addr().String()}
	params.mgmtEndpoint.Insecure = true

	const workers = 8

	type result struct {
		client *Client
		err    error
	}

	done := make(chan result, workers)
	for range workers {
		go func() { client, err := cache.GetOrCreate(params); done <- result{client, err} }()
	}

	first := <-done
	require.NoError(t, first.err)

	for range workers - 1 {
		got := <-done
		require.NoError(t, got.err)
		assert.Same(t, first.client, got.client)
	}

	assert.Equal(t, int32(1), authCalls.Load(), "one constructor authentication")
	assert.Equal(t, int32(1), connections.Load(), "one TLS connection")
	cache.Delete(params)
	<-closed
}
