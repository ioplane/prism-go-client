package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	prism "github.com/nutanix-cloud-native/prism-go-client"
)

func sessionClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(WithCredentials(&prism.Credentials{URL: server.URL, SessionAuth: true}),
		WithUserAgent(testUserAgent), WithAbsolutePath(testAbsolutePath))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func sessionDo(client *Client, ctx context.Context, request *http.Request, filtered bool) error {
	var result map[string]any
	if filtered {
		return client.DoWithFilters(ctx, request, &result, nil, nil)
	}
	return client.Do(ctx, request, &result)
}

func TestSessionConcurrentExpiration(t *testing.T) {
	const workers = 8
	var authCalls, expiredCalls, resourceCalls atomic.Int32
	expired := make(chan struct{})
	client := sessionClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/users/me") {
			call := authCalls.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "session-id", Value: fmt.Sprint(call), Path: "/"})
			_, _ = io.WriteString(w, `{}`)
			return
		}
		resourceCalls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != "{\"kind\":\"project\"}\n" {
			t.Errorf("body=%q error=%v", body, err)
		}
		cookie, err := r.Cookie("session-id")
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if cookie.Value == "1" {
			if expiredCalls.Add(1) == workers {
				close(expired)
			}
			<-expired
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"entities":[]}`)
	})
	results := make(chan error, workers)
	for index := range workers {
		go func() {
			request, err := client.NewRequest(http.MethodPost, "/projects/list", map[string]string{"kind": "project"})
			if err == nil {
				err = sessionDo(client, context.Background(), request, index%2 == 0)
			}
			results <- err
		}()
	}
	for range workers {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	if got := authCalls.Load(); got != 2 {
		t.Errorf("auth calls=%d, want2 including constructor", got)
	}
	if got := resourceCalls.Load(); got != 2*workers {
		t.Errorf("resource calls=%d, want%d", got, 2*workers)
	}
}

func TestSessionNonReplayableBody(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(fmt.Sprint(filtered), func(t *testing.T) {
			var authCalls, resourceCalls atomic.Int32
			client := sessionClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/users/me") {
					authCalls.Add(1)
					http.SetCookie(w, &http.Cookie{Name: "session-id", Value: "initial"})
					return
				}
				resourceCalls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.WriteHeader(http.StatusUnauthorized)
			})
			request, err := http.NewRequest(http.MethodPost, client.BaseURL.String()+"operation", io.NopCloser(strings.NewReader("payload")))
			if err != nil {
				t.Fatal(err)
			}
			err = sessionDo(client, context.Background(), request, filtered)
			var responseError *prism.ResponseError
			if !errors.As(err, &responseError) || responseError.StatusCode != http.StatusUnauthorized {
				t.Fatalf("error=%v, want401", err)
			}
			if got := resourceCalls.Load(); got != 1 {
				t.Errorf("resource calls=%d, want1", got)
			}
			if got := authCalls.Load(); got != 1 {
				t.Errorf("auth calls=%d, want1", got)
			}
		})
	}
}

func TestSessionRetryLimit(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(fmt.Sprint(filtered), func(t *testing.T) {
			var authCalls, resourceCalls atomic.Int32
			client := sessionClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/users/me") {
					http.SetCookie(w, &http.Cookie{Name: "session-id", Value: fmt.Sprint(authCalls.Add(1))})
					return
				}
				resourceCalls.Add(1)
				w.WriteHeader(http.StatusUnauthorized)
			})
			request, err := client.NewRequest(http.MethodGet, "/projects/list", nil)
			if err != nil {
				t.Fatal(err)
			}
			err = sessionDo(client, context.Background(), request, filtered)
			var responseError *prism.ResponseError
			if !errors.As(err, &responseError) || responseError.StatusCode != http.StatusUnauthorized {
				t.Fatalf("error=%v, want401", err)
			}
			if got := resourceCalls.Load(); got != 2 {
				t.Errorf("resource calls=%d,want2", got)
			}
			if got := authCalls.Load(); got != 2 {
				t.Errorf("auth calls=%d,want2", got)
			}
		})
	}
}

func TestSessionRefreshWaitCancellation(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(fmt.Sprint(filtered), func(t *testing.T) {
			refreshStarted, releaseRefresh, waiterSent := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var authCalls atomic.Int32
			client := sessionClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/users/me") {
					call := authCalls.Add(1)
					if call == 2 {
						close(refreshStarted)
						<-releaseRefresh
					}
					http.SetCookie(w, &http.Cookie{Name: "session-id", Value: fmt.Sprint(call)})
					return
				}
				cookie, _ := r.Cookie("session-id")
				if cookie != nil && cookie.Value == "1" {
					if r.Header.Get("Waiter") == "yes" {
						close(waiterSent)
					}
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = io.WriteString(w, `{"entities":[]}`)
			})
			// Release the handler before server cleanup even if an assertion fails.
			defer close(releaseRefresh)
			request, err := client.NewRequest(http.MethodGet, "/projects/list", nil)
			require.NoError(t, err)
			originalCookies := request.Header.Get("Cookie")
			leader := make(chan error, 1)
			go func() { leader <- sessionDo(client, context.Background(), request, false) }()
			<-refreshStarted
			waiterRequest, err := client.NewRequest(http.MethodGet, "/projects/list", nil)
			require.NoError(t, err)
			waiterRequest.Header.Set("Waiter", "yes")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			waiter := make(chan error, 1)
			go func() { waiter <- sessionDo(client, ctx, waiterRequest, filtered) }()
			<-waiterSent
			cancel()
			require.ErrorIs(t, <-waiter, context.Canceled)
			assert.Equal(t, int32(2), authCalls.Load(), "auth calls")
			// The canceled waiter must return while the leader's refresh is blocked.
			releaseRefresh <- struct{}{}
			require.NoError(t, <-leader)
			assert.Equal(t, originalCookies, request.Header.Get("Cookie"), "caller cookies")
		})
	}
}

func TestSessionTransportErrorHasNoReplay(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(fmt.Sprint(filtered), func(t *testing.T) {
			want := errors.New("unknown operation outcome")
			var calls int
			client := responseClient(nil, 0)
			client.credentials.SessionAuth = true
			client.httpClient.Transport = responseTransport(func(request *http.Request) (*http.Response, error) { calls++; return nil, want })
			request, err := http.NewRequest(http.MethodPost, "http://fixture.invalid", strings.NewReader("payload"))
			if err != nil {
				t.Fatal(err)
			}
			if err := sessionDo(client, context.Background(), request, filtered); !errors.Is(err, want) {
				t.Fatalf("error=%v", err)
			}
			if calls != 1 {
				t.Errorf("calls=%d,want1", calls)
			}
		})
	}
}

func TestSessionReplayFailure(t *testing.T) {
	for _, testCase := range []struct{ filtered, authFailure bool }{
		{false, false}, {false, true}, {true, false}, {true, true},
	} {
		filtered, authFailure := testCase.filtered, testCase.authFailure
		t.Run(fmt.Sprintf("filtered=%t/authFailure=%t", filtered, authFailure), func(t *testing.T) {
			var authCalls, resourceCalls atomic.Int32
			client := sessionClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/users/me") {
					if authCalls.Add(1) > 1 && authFailure {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					http.SetCookie(w, &http.Cookie{Name: "session-id", Value: "session"})
					return
				}
				resourceCalls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.WriteHeader(http.StatusUnauthorized)
			})
			request, err := client.NewRequest(http.MethodPost, "/operation", map[string]string{"kind": "project"})
			require.NoError(t, err)
			want := errors.New("replay unavailable")
			request.GetBody = func() (io.ReadCloser, error) { return nil, want }
			err = sessionDo(client, context.Background(), request, filtered)
			if authFailure {
				var responseError *prism.ResponseError
				require.ErrorAs(t, err, &responseError)
				assert.Equal(t, http.StatusServiceUnavailable, responseError.StatusCode)
			} else {
				require.ErrorIs(t, err, want)
			}
			assert.Equal(t, int32(1), resourceCalls.Load(), "resource calls")
			assert.Equal(t, int32(2), authCalls.Load(), "auth calls")
		})
	}
}

func TestSessionResponseOwnership(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(fmt.Sprint(filtered), func(t *testing.T) {
			_, client, server := setup(t)
			t.Cleanup(server.Close)
			client.credentials.SessionAuth = true
			bodies := []*responseBody{
				{Reader: strings.NewReader(`{}`)},
				{Reader: strings.NewReader(`{}`)},
				{Reader: strings.NewReader(`{"entities":[]}`)},
			}
			calls := 0
			client.httpClient.Transport = responseTransport(func(request *http.Request) (*http.Response, error) {
				index := calls
				calls++
				if index >= len(bodies) {
					t.Fatal("unexpected request")
				}
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
			request, err := client.NewRequest(http.MethodGet, "/projects/list", nil)
			require.NoError(t, err)
			require.NoError(t, sessionDo(client, context.Background(), request, filtered))
			assert.Equal(t, 3, calls, "requests")
			for index, body := range bodies {
				assert.Equal(t, 1, body.closes, "body%d closes", index)
			}
		})
	}
}
