package internal_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	prism "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/internal"
	"github.com/nutanix-cloud-native/prism-go-client/v3"
)

func baselinePayload(b *testing.B, count int) []byte {
	b.Helper()
	response := v3.VMListIntentResponse{Entities: make([]*v3.VMIntentResource, count)}
	for i := range response.Entities {
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
		response.Entities[i] = &v3.VMIntentResource{Metadata: &v3.Metadata{UUID: &id}}
	}
	payload, err := json.Marshal(response)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("fixture_sha256=%x bytes=%d entities=%d", sha256.Sum256(payload), len(payload), count)
	return payload
}

type baselineTransport struct {
	payload []byte
	calls   int
	auth    int
}

func (t *baselineTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.calls++
	header := make(http.Header)
	if strings.HasSuffix(r.URL.Path, "/users/me") {
		t.auth++
		header.Set("Set-Cookie", "fixture-session=offline; HttpOnly")
	}
	return &http.Response{StatusCode: http.StatusOK, Header: header,
		Body: io.NopCloser(bytes.NewReader(t.payload)), Request: r}, nil
}

func BenchmarkClientSessionAuth(b *testing.B) {
	transport := &baselineTransport{}
	logger := logr.Discard()
	credentials := &prism.Credentials{URL: "https://fixture.invalid", SessionAuth: true}
	b.ReportAllocs()
	for b.Loop() {
		_, err := internal.NewClient(internal.WithCredentials(credentials),
			internal.WithUserAgent("offline-baseline"), internal.WithAbsolutePath("api/nutanix/v3"),
			internal.WithLogger(&logger), internal.WithRoundTripper(transport))
		if err != nil {
			b.Fatal(err)
		}
	}
	if transport.auth != b.N+1 || transport.calls != b.N {
		b.Fatalf("auth=%d requests=%d, want %d each", transport.auth, transport.calls, b.N)
	}
	b.ReportMetric(float64(transport.auth)/float64(b.N), "auth/op")
}

func baselineClient(b *testing.B, endpoint string, transport http.RoundTripper) *internal.Client {
	b.Helper()
	logger := logr.Discard()
	client, err := internal.NewClient(internal.WithCredentials(&prism.Credentials{URL: endpoint}),
		internal.WithUserAgent("offline-baseline"), internal.WithAbsolutePath("api/nutanix/v3"),
		internal.WithLogger(&logger), internal.WithRoundTripper(transport))
	if err != nil {
		b.Fatal(err)
	}
	return client
}

func baselineRequest(b *testing.B, client *internal.Client, ctx context.Context, count int) {
	request, err := client.NewRequest(http.MethodPost, "vms/list", &v3.DSMetadata{})
	if err != nil {
		b.Fatal(err)
	}
	var response v3.VMListIntentResponse
	if err := client.Do(ctx, request, &response); err != nil {
		b.Fatal(err)
	}
	if len(response.Entities) != count {
		b.Fatalf("decoded entities=%d, want %d", len(response.Entities), count)
	}
}

func BenchmarkClientDecode(b *testing.B) {
	for _, count := range []int{1, 128} {
		b.Run(fmt.Sprintf("entities=%d", count), func(b *testing.B) {
			payload := baselinePayload(b, count)
			transport := &baselineTransport{payload: payload}
			client := baselineClient(b, "https://fixture.invalid", transport)
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			for b.Loop() {
				baselineRequest(b, client, context.Background(), count)
			}
			if transport.calls != b.N {
				b.Fatalf("requests=%d, want %d", transport.calls, b.N)
			}
			b.ReportMetric(float64(transport.calls)/float64(b.N), "requests/op")
			b.ReportMetric(float64(len(payload)), "body-bytes/op")
		})
	}
}

// This local TLS experiment measures connection policy, not Prism latency.
func BenchmarkClientTLS(b *testing.B) {
	for _, disableKeepAlive := range []bool{false, true} {
		b.Run(fmt.Sprintf("disable-keepalive=%t", disableKeepAlive), func(b *testing.B) {
			payload := baselinePayload(b, 1)
			var calls, handshakes atomic.Int64
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					b.Error(err)
				}
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(payload); err != nil {
					b.Error(err)
				}
			}))
			b.Cleanup(server.Close)
			transport := server.Client().Transport.(*http.Transport).Clone()
			transport.DisableKeepAlives = disableKeepAlive
			b.Cleanup(transport.CloseIdleConnections)
			client := baselineClient(b, server.URL, transport)
			ctx := httptrace.WithClientTrace(context.Background(), &httptrace.ClientTrace{
				TLSHandshakeDone: func(tls.ConnectionState, error) { handshakes.Add(1) },
			})
			b.ReportAllocs()
			for b.Loop() {
				baselineRequest(b, client, ctx, 1)
			}
			wantHandshakes := int64(1)
			if disableKeepAlive {
				wantHandshakes = int64(b.N)
			}
			if calls.Load() != int64(b.N) || handshakes.Load() != wantHandshakes {
				b.Fatalf("requests=%d handshakes=%d, want %d/%d", calls.Load(), handshakes.Load(), b.N, wantHandshakes)
			}
			b.ReportMetric(float64(calls.Load())/float64(b.N), "requests/op")
			b.ReportMetric(float64(len(payload)), "body-bytes/op")
			b.ReportMetric(float64(handshakes.Load())/float64(b.N), "TLS/op")
		})
	}
}
