package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckResponseRejectsEveryNon2xx(t *testing.T) {
	t.Parallel()

	for _, code := range []int{
		http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound,
		http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable,
	} {
		for index, body := range []string{
			"", testEmptyObject, `{"status":"ERROR"}`, `{"status":null}`,
			testEmptyEntities, `{"state":"COMPLETE"}`, `{"status":42}`, testBrokenJSON, "proxy error",
		} {
			t.Run(fmt.Sprintf("%d/body%d", code, index), func(t *testing.T) {
				t.Parallel()

				response := &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}

				var responseError *prismgoclient.ResponseError
				if err := CheckResponse(response); !errors.As(err, &responseError) || responseError.StatusCode != code {
					t.Fatalf("HTTP %d lost typed status: %v", code, err)
				}
			})
		}
	}
}

type responseBody struct {
	io.Reader

	closes     int
	closeError error
}

func (body *responseBody) Close() error {
	body.closes++

	return body.closeError
}

type responseTransport func(*http.Request) (*http.Response, error)

func (transport responseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func responseClient(body *responseBody, code int) *Client {
	client := &Client{}
	client.credentials = &prismgoclient.Credentials{}
	client.httpClient = &http.Client{Transport: responseTransport(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: code, Header: make(http.Header), Body: body, Request: request}, nil
	})}

	return client
}

func TestResponseOriginalBodyClosed(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		code      int
		body      string
		filters   []*prismgoclient.AdditionalFilter
		wantError bool
	}{
		{
			name: "filtered", code: http.StatusOK, body: `{"entities":[{"name":"keep"}]}`,
			filters: []*prismgoclient.AdditionalFilter{{Name: testName, Values: []string{testKeep}}},
		},
		{
			name: "empty filters", code: http.StatusOK, body: testEmptyEntities,
			filters: []*prismgoclient.AdditionalFilter{},
		},
		{
			name: "filter decode error", code: http.StatusOK, body: testBrokenJSON,
			filters: []*prismgoclient.AdditionalFilter{{Name: testName, Values: []string{testKeep}}}, wantError: true,
		},
		{name: "result decode error", code: http.StatusOK, body: testBrokenJSON, wantError: true},
		{name: "http error", code: http.StatusTooManyRequests, body: testEmptyObject, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := &responseBody{Reader: strings.NewReader(testCase.body)}
			client := responseClient(body, testCase.code)

			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testEndpoint, http.NoBody)
			require.NoError(t, err)

			var result map[string]any

			err = client.DoWithFilters(context.Background(), request, &result, testCase.filters, nil)
			if (err != nil) != testCase.wantError {
				t.Errorf("error=%v, wantError=%v", err, testCase.wantError)
			}

			if body.closes != 1 {
				t.Errorf("original body Close calls=%d, want1", body.closes)
			}
		})
	}
}

func TestDoClosesOriginalHTTPErrorBody(t *testing.T) {
	t.Parallel()

	body := &responseBody{Reader: strings.NewReader(testEmptyObject)}
	client := responseClient(body, http.StatusInternalServerError)

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testEndpoint, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Do(context.Background(), request, nil); err == nil {
		t.Error("expected HTTP error")
	}

	if body.closes != 1 {
		t.Errorf("original body Close calls=%d, want1", body.closes)
	}
}

func TestDoWithFiltersReturnsCloseError(t *testing.T) {
	t.Parallel()

	closeError := errors.New("fixture-secret-close-error")
	body := &responseBody{Reader: strings.NewReader(testEmptyEntities), closeError: closeError}
	client := responseClient(body, http.StatusOK)

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testEndpoint, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	err = client.DoWithFilters(context.Background(), request, nil, nil, nil)
	if !errors.Is(err, closeError) {
		t.Errorf("close error was lost: %v", err)
	}

	if strings.Contains(fmt.Sprint(err), closeError.Error()) {
		t.Error("close error text leaked")
	}
}

func TestDecodeErrorCauseRetained(t *testing.T) {
	t.Parallel()

	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			t.Parallel()

			body := &responseBody{Reader: strings.NewReader("{")}
			client := responseClient(body, http.StatusOK)

			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testEndpoint, http.NoBody)
			require.NoError(t, err)

			var result map[string]any
			if filtered {
				err = client.DoWithFilters(t.Context(), request, &result, nil, nil)
			} else {
				err = client.Do(t.Context(), request, &result)
			}

			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("decoder error cause lost: %v", err)
			}
		})
	}
}

func TestFilterRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	filters := []*prismgoclient.AdditionalFilter{{Name: testName, Values: []string{testKeep}}}

	for _, body := range []string{
		testEmptyObject, `{"entities":null}`, `{"entities":{}}`,
		`{"entities":"invalid"}`, `{"entities":[null]}`, `{"entities":[42]}`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			_, err := filter(io.NopCloser(strings.NewReader(body)), filters, nil)
			if err == nil {
				t.Fatal("expected invalid entities error")
			}
		})
	}

	t.Run("nil filter", func(t *testing.T) {
		t.Parallel()

		_, err := filter(io.NopCloser(strings.NewReader(testEmptyEntities)),
			[]*prismgoclient.AdditionalFilter{nil}, nil)
		if err == nil {
			t.Fatal("expected nil filter error")
		}
	})
}

func TestHTTPErrorDoesNotExposeResponse(t *testing.T) {
	const marker = testSecretMarker

	var logs bytes.Buffer

	previous := log.Writer()

	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	for _, body := range []string{
		marker,
		`{"message":"` + marker + `"}`,
		`{"message_info":"` + marker + `"}`,
		`{"state":"ERROR","message_list":[{"message":"` + marker + `","reason":"` + marker + `","details":"` + marker + `"}]}`,
		`{"status":{"state":"ERROR","message_list":[{"message":"` + marker + `"}]}}`,
	} {
		response := &http.Response{
			StatusCode: http.StatusBadRequest,
			Status:     marker, Body: io.NopCloser(strings.NewReader(body)),
		}

		err := CheckResponse(response)
		if err == nil || strings.Contains(fmt.Sprint(err), marker) {
			t.Errorf("unsafe HTTP error: %v", err)
		}
	}

	if strings.Contains(logs.String(), marker) {
		t.Error("response marker leaked to log")
	}
}

func TestResponseReadBudget(t *testing.T) {
	const (
		jsonLimit  = 64 << 20
		errorLimit = 64 << 10
	)

	largeJSON := `{"entities":[],"padding":"` + strings.Repeat("x", jsonLimit) + `"}`
	for _, testCase := range []struct {
		name     string
		code     int
		payload  string
		limit    int
		filtered bool
	}{
		{name: "decoded JSON", code: http.StatusOK, payload: largeJSON, limit: jsonLimit},
		{name: "filtered JSON", code: http.StatusOK, payload: largeJSON, limit: jsonLimit, filtered: true},
		{name: "error body", code: http.StatusBadRequest, payload: strings.Repeat("x", errorLimit+2), limit: errorLimit},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			reader := strings.NewReader(testCase.payload)
			body := &responseBody{Reader: reader}
			client := responseClient(body, testCase.code)

			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testEndpoint, http.NoBody)
			require.NoError(t, err)

			var result map[string]any
			if testCase.filtered {
				err = client.DoWithFilters(t.Context(), request, &result,
					[]*prismgoclient.AdditionalFilter{{Name: testName, Values: []string{testKeep}}}, nil)
			} else {
				err = client.Do(t.Context(), request, &result)
			}

			var responseError *prismgoclient.ResponseError

			require.ErrorIs(t, err, prismgoclient.ErrResponseTooLarge)
			require.ErrorAs(t, err, &responseError)
			assert.Equal(t, testCase.code, responseError.StatusCode)

			assert.LessOrEqual(t, len(testCase.payload)-reader.Len(), testCase.limit+1)

			assert.Equal(t, 1, body.closes)
		})
	}
}

func cancellationServer(t *testing.T, reading chan struct{}) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, err := io.WriteString(writer, `{"entities":[`); err != nil {
			t.Error(err)

			return
		}

		flusher, ok := writer.(http.Flusher)
		if !ok {
			t.Error("response writer cannot flush")

			return
		}

		flusher.Flush()
		close(reading)
		<-request.Context().Done()
	}))
}

func TestResponseCancellation(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			reading := make(chan struct{})

			server := cancellationServer(t, reading)
			defer server.Close()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			client := &Client{}
			client.credentials = &prismgoclient.Credentials{}
			client.httpClient = server.Client()

			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
			require.NoError(t, err)

			done := make(chan error, 1)

			go func() {
				var result map[string]any
				if filtered {
					done <- client.DoWithFilters(ctx, request, &result,
						[]*prismgoclient.AdditionalFilter{{Name: testName}}, nil)
				} else {
					done <- client.Do(ctx, request, &result)
				}
			}()

			<-reading
			cancel()

			require.ErrorIs(t, <-done, context.Canceled)
		})
	}
}

func TestSafeDecodeErrorsAndStreaming(t *testing.T) {
	const marker = testSecretMarker

	for _, filtered := range []bool{false, true} {
		var result struct {
			Value string `json:"fixture-secret-marker"`
		}

		body := &responseBody{Reader: strings.NewReader(`{"fixture-secret-marker":123}`)}
		client := responseClient(body, http.StatusOK)

		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testEndpoint, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}

		if filtered {
			err = client.DoWithFilters(t.Context(), request, &result, nil, nil)
		} else {
			err = client.Do(t.Context(), request, &result)
		}

		var typeError *json.UnmarshalTypeError
		require.ErrorAs(t, err, &typeError)
		assert.NotContains(t, fmt.Sprint(err), marker)

		body = &responseBody{Reader: strings.NewReader(marker)}
		client = responseClient(body, http.StatusOK)

		var output strings.Builder
		if filtered {
			err = client.DoWithFilters(t.Context(), request, &output, nil, nil)
		} else {
			err = client.Do(t.Context(), request, &output)
		}

		require.NoError(t, err)
		assert.Equal(t, marker, output.String())
		assert.Equal(t, 1, body.closes)
	}

	const readLimit = 8
	for _, size := range []int{readLimit, readLimit + 1} {
		_, err := readResponseBody(strings.NewReader(strings.Repeat("x", size)), readLimit)
		if (size == readLimit && err != nil) || (size == readLimit+1 && !errors.Is(err, prismgoclient.ErrResponseTooLarge)) {
			t.Errorf("size %d: incorrect boundary error %v", size, err)
		}
	}
}
