package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
)

func sessionClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(WithCredentials(&prismgoclient.Credentials{URL: server.URL, SessionAuth: true}),
		WithUserAgent(testUserAgent), WithAbsolutePath(testAbsolutePath))
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func sessionDo(ctx context.Context, client *Client, request *http.Request, filtered bool) error {
	var result map[string]any
	if filtered {
		return client.DoWithFilters(ctx, request, &result, nil, nil)
	}

	return client.Do(ctx, request, &result)
}

type sessionExpirationServer struct {
	test          *testing.T
	workers       int32
	authCalls     atomic.Int32
	expiredCalls  atomic.Int32
	resourceCalls atomic.Int32
	expired       chan struct{}
}

func (fixture *sessionExpirationServer) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	if strings.HasSuffix(request.URL.Path, "/users/me") {
		call := fixture.authCalls.Add(1)
		http.SetCookie(writer, &http.Cookie{Name: testSessionID, Value: strconv.Itoa(int(call)), Path: testRootPath})
		writeTestResponse(fixture.test, writer, testEmptyObject)

		return
	}

	fixture.resourceCalls.Add(1)

	body, err := io.ReadAll(request.Body)
	if err != nil {
		fixture.test.Error(err)

		return
	}

	if string(body) != "{\"kind\":\"project\"}\n" {
		fixture.test.Errorf("body=%q", body)
	}

	cookie, err := request.Cookie(testSessionID)
	if err != nil {
		fixture.test.Error(err)
		writer.WriteHeader(http.StatusBadRequest)

		return
	}

	if cookie.Value == "1" {
		if fixture.expiredCalls.Add(1) == fixture.workers {
			close(fixture.expired)
		}

		<-fixture.expired
		writer.WriteHeader(http.StatusUnauthorized)

		return
	}

	writeTestResponse(fixture.test, writer, testEmptyEntities)
}

func TestSessionConcurrentExpiration(t *testing.T) {
	const workers = 8

	fixture := &sessionExpirationServer{
		test: t, workers: workers, expired: make(chan struct{}),
		authCalls: atomic.Int32{}, expiredCalls: atomic.Int32{}, resourceCalls: atomic.Int32{},
	}
	client := sessionClient(t, fixture.serveHTTP)

	results := make(chan error, workers)
	for index := range workers {
		go func() {
			request, err := client.NewRequest(http.MethodPost, testProjectsPath, map[string]string{testKind: testProject})
			if err == nil {
				err = sessionDo(context.Background(), client, request, index%2 == 0)
			}

			results <- err
		}()
	}

	failures := make([]error, workers)
	for index := range workers {
		failures[index] = <-results
	}

	require.NoError(t, errors.Join(failures...))

	assert.Equal(t, int32(2), fixture.authCalls.Load())

	assert.Equal(t, int32(2*workers), fixture.resourceCalls.Load())
}

func TestSessionNonReplayableBody(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			var authCalls, resourceCalls atomic.Int32

			client := sessionClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if strings.HasSuffix(request.URL.Path, "/users/me") {
					authCalls.Add(1)
					http.SetCookie(writer, &http.Cookie{Name: testSessionID, Value: "initial"})

					return
				}

				resourceCalls.Add(1)

				if _, err := io.Copy(io.Discard, request.Body); err != nil {
					t.Error(err)

					return
				}

				writer.WriteHeader(http.StatusUnauthorized)
			})

			request, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
				client.BaseURL.String()+"operation", io.NopCloser(strings.NewReader("payload")))
			require.NoError(t, err)

			err = sessionDo(context.Background(), client, request, filtered)

			var responseError *prismgoclient.ResponseError
			require.ErrorAs(t, err, &responseError)
			require.Equal(t, http.StatusUnauthorized, responseError.StatusCode)

			assert.Equal(t, int32(1), resourceCalls.Load())

			assert.Equal(t, int32(1), authCalls.Load())
		})
	}
}

func TestSessionRetryLimit(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			var authCalls, resourceCalls atomic.Int32

			client := sessionClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if strings.HasSuffix(request.URL.Path, "/users/me") {
					http.SetCookie(writer, &http.Cookie{Name: testSessionID, Value: strconv.Itoa(int(authCalls.Add(1)))})

					return
				}

				resourceCalls.Add(1)
				writer.WriteHeader(http.StatusUnauthorized)
			})

			request, err := client.NewRequest(http.MethodGet, testProjectsPath, nil)
			require.NoError(t, err)

			err = sessionDo(context.Background(), client, request, filtered)

			var responseError *prismgoclient.ResponseError
			require.ErrorAs(t, err, &responseError)
			require.Equal(t, http.StatusUnauthorized, responseError.StatusCode)

			assert.Equal(t, int32(2), resourceCalls.Load())

			assert.Equal(t, int32(2), authCalls.Load())
		})
	}
}

func TestSessionRefreshWaitCancellation(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			sessionRefreshWaitCancellation(t, filtered)
		})
	}
}

func sessionRefreshWaitCancellation(t *testing.T, filtered bool) {
	t.Helper()

	refreshStarted, releaseRefresh, waiterSent := make(chan struct{}), make(chan struct{}), make(chan struct{})

	var authCalls atomic.Int32

	client := sessionClient(t, func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/users/me") {
			call := authCalls.Add(1)
			if call == 2 {
				close(refreshStarted)
				<-releaseRefresh
			}

			http.SetCookie(writer, &http.Cookie{Name: testSessionID, Value: strconv.Itoa(int(call))})

			return
		}

		cookie, err := request.Cookie(testSessionID)
		if err != nil {
			t.Error(err)

			return
		}

		if cookie != nil && cookie.Value == "1" {
			if request.Header.Get("Waiter") == "yes" {
				close(waiterSent)
			}

			writer.WriteHeader(http.StatusUnauthorized)

			return
		}

		writeTestResponse(t, writer, testEmptyEntities)
	})
	// Release the handler before server cleanup even if an assertion fails.
	defer close(releaseRefresh)

	request, err := client.NewRequest(http.MethodGet, testProjectsPath, nil)
	require.NoError(t, err)

	originalCookies := request.Header.Get("Cookie")

	leader := make(chan error, 1)
	go func() { leader <- sessionDo(context.Background(), client, request, false) }()

	<-refreshStarted

	waiterRequest, err := client.NewRequest(http.MethodGet, testProjectsPath, nil)
	require.NoError(t, err)
	waiterRequest.Header.Set("Waiter", "yes")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	waiter := make(chan error, 1)
	go func() { waiter <- sessionDo(ctx, client, waiterRequest, filtered) }()

	<-waiterSent
	cancel()
	require.ErrorIs(t, <-waiter, context.Canceled)
	assert.Equal(t, int32(2), authCalls.Load(), "auth calls")
	// The canceled waiter must return while the leader's refresh is blocked.
	releaseRefresh <- struct{}{}

	require.NoError(t, <-leader)
	assert.Equal(t, originalCookies, request.Header.Get("Cookie"), "caller cookies")
}

func TestSessionTransportErrorHasNoReplay(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			want := errors.New("unknown operation outcome")

			var calls int

			client := responseClient(nil, 0)
			client.credentials.SessionAuth = true
			client.httpClient.Transport = responseTransport(func(_ *http.Request) (*http.Response, error) {
				calls++

				return nil, want
			})

			request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, testEndpoint, strings.NewReader("payload"))
			require.NoError(t, err)

			if err := sessionDo(context.Background(), client, request, filtered); !errors.Is(err, want) {
				t.Fatalf("error=%v", err)
			}

			if calls != 1 {
				t.Errorf("calls=%d,want1", calls)
			}
		})
	}
}

type sessionReplayServer struct {
	test          *testing.T
	authFailure   bool
	authCalls     atomic.Int32
	resourceCalls atomic.Int32
}

func (fixture *sessionReplayServer) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	if strings.HasSuffix(request.URL.Path, "/users/me") {
		if fixture.authCalls.Add(1) > 1 && fixture.authFailure {
			writer.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		http.SetCookie(writer, &http.Cookie{Name: testSessionID, Value: "session"})

		return
	}

	fixture.resourceCalls.Add(1)

	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		fixture.test.Error(err)

		return
	}

	writer.WriteHeader(http.StatusUnauthorized)
}

func TestSessionReplayFailure(t *testing.T) {
	for _, testCase := range []struct{ filtered, authFailure bool }{
		{false, false}, {false, true}, {true, false}, {true, true},
	} {
		filtered, authFailure := testCase.filtered, testCase.authFailure
		t.Run(fmt.Sprintf("filtered=%t/authFailure=%t", filtered, authFailure), func(t *testing.T) {
			fixture := &sessionReplayServer{
				test: t, authFailure: authFailure,
				authCalls: atomic.Int32{}, resourceCalls: atomic.Int32{},
			}
			client := sessionClient(t, fixture.serveHTTP)
			request, err := client.NewRequest(http.MethodPost, "/operation", map[string]string{testKind: testProject})
			require.NoError(t, err)

			want := errors.New("replay unavailable")
			request.GetBody = func() (io.ReadCloser, error) { return nil, want }
			err = sessionDo(context.Background(), client, request, filtered)

			if authFailure {
				var responseError *prismgoclient.ResponseError
				require.ErrorAs(t, err, &responseError)
				assert.Equal(t, http.StatusServiceUnavailable, responseError.StatusCode)
			} else {
				require.ErrorIs(t, err, want)
			}

			assert.Equal(t, int32(1), fixture.resourceCalls.Load(), "resource calls")
			assert.Equal(t, int32(2), fixture.authCalls.Load(), "auth calls")
		})
	}
}

func TestSessionResponseOwnership(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			_, client, server := setup(t)
			t.Cleanup(server.Close)

			client.credentials.SessionAuth = true
			bodies := []*responseBody{
				{Reader: strings.NewReader(testEmptyObject)},
				{Reader: strings.NewReader(testEmptyObject)},
				{Reader: strings.NewReader(testEmptyEntities)},
			}
			calls := 0
			client.httpClient.Transport = responseTransport(func(request *http.Request) (*http.Response, error) {
				index := calls
				calls++

				require.Less(t, index, len(bodies), "unexpected request")

				status := http.StatusOK
				if index == 0 {
					status = http.StatusUnauthorized
				}

				header := make(http.Header)
				if index == 1 {
					header.Set("Set-Cookie", "session-id=refreshed")
				}

				return &http.Response{StatusCode: status, Header: header, Body: bodies[index], Request: request}, nil
			})
			request, err := client.NewRequest(http.MethodGet, testProjectsPath, nil)
			require.NoError(t, err)
			require.NoError(t, sessionDo(context.Background(), client, request, filtered))
			assert.Equal(t, 3, calls, "requests")

			for index, body := range bodies {
				assert.Equal(t, 1, body.closes, "body%d closes", index)
			}
		})
	}
}

func TestSessionUnauthenticatedRequestHeaders(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			headers := make(chan string, 1)
			client := sessionClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if strings.HasSuffix(request.URL.Path, "/users/me") {
					http.SetCookie(writer, &http.Cookie{Name: testSessionID, Value: "session"})

					return
				}

				headers <- request.Header.Get("Cookie")

				writeTestResponse(t, writer, testEmptyEntities)
			})
			request, err := client.NewUnAuthRequest(http.MethodGet, testProjectsPath, nil)
			require.NoError(t, err)
			request.Header.Set("Cookie", "caller=value")
			require.NoError(t, sessionDo(context.Background(), client, request, filtered))
			assert.Equal(t, "caller=value", <-headers, "unauthenticated headers on wire")
			assert.Equal(t, "caller=value", request.Header.Get("Cookie"), "caller headers")
		})
	}
}

func TestSessionPreviouslyBuiltRequest(t *testing.T) {
	const expectedResourceCalls = 4

	var authCalls, resourceCalls atomic.Int32

	client := sessionClient(t, func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/users/me") {
			http.SetCookie(writer, &http.Cookie{Name: testSessionID, Value: strconv.Itoa(int(authCalls.Add(1)))})

			return
		}

		resourceCalls.Add(1)

		cookie, err := request.Cookie(testSessionID)
		if err != nil || cookie.Value == "1" {
			writer.WriteHeader(http.StatusUnauthorized)

			return
		}

		writeTestResponse(t, writer, testEmptyEntities)
	})
	request, err := client.NewRequest(http.MethodGet, testProjectsPath, nil)
	require.NoError(t, err)
	queued, err := client.NewRequest(http.MethodGet, testProjectsPath, nil)
	require.NoError(t, err)
	require.NoError(t, sessionDo(context.Background(), client, request, false))
	require.NoError(t, sessionDo(context.Background(), client, queued, true))
	assert.Equal(t, int32(2), authCalls.Load(), "stale queued cookies reuse completed refresh")
	assert.Equal(t, int32(expectedResourceCalls), resourceCalls.Load(), "original cookie headers reach both first attempts")
}
