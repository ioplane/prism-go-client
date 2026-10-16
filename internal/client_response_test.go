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
	"strings"
	"testing"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
)

func TestCheckResponseRejectsEveryNon2xx(t *testing.T) {
	t.Parallel()

	for _, code := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound,
		http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		for index, body := range []string{"", `{}`, `{"status":"ERROR"}`, `{"status":null}`,
			`{"entities":[]}`, `{"state":"COMPLETE"}`, `{"status":42}`, `{"broken":`, "proxy error"} {
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
	return &Client{
		credentials: &prismgoclient.Credentials{},
		httpClient: &http.Client{Transport: responseTransport(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: code, Header: make(http.Header), Body: body, Request: request}, nil
		})},
	}
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
		{name: "filtered", code: http.StatusOK, body: `{"entities":[{"name":"keep"}]}`,
			filters: []*prismgoclient.AdditionalFilter{{Name: "name", Values: []string{"keep"}}}},
		{name: "empty filters", code: http.StatusOK, body: `{"entities":[]}`,
			filters: []*prismgoclient.AdditionalFilter{}},
		{name: "filter decode error", code: http.StatusOK, body: `{"broken":`,
			filters: []*prismgoclient.AdditionalFilter{{Name: "name", Values: []string{"keep"}}}, wantError: true},
		{name: "result decode error", code: http.StatusOK, body: `{"broken":`, wantError: true},
		{name: "http error", code: http.StatusTooManyRequests, body: `{}`, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := &responseBody{Reader: strings.NewReader(testCase.body)}
			client := responseClient(body, testCase.code)
			request, err := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
			if err != nil {
				t.Fatal(err)
			}

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

	body := &responseBody{Reader: strings.NewReader(`{}`)}
	client := responseClient(body, http.StatusInternalServerError)
	request, err := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
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
	body := &responseBody{Reader: strings.NewReader(`{"entities":[]}`), closeError: closeError}
	client := responseClient(body, http.StatusOK)
	request, err := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
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
		t.Run(fmt.Sprint(filtered), func(t *testing.T) {
			t.Parallel()
			body := &responseBody{Reader: strings.NewReader("{")}
			client := responseClient(body, http.StatusOK)
			request, err := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
			if err != nil {
				t.Fatal(err)
			}
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

	filters := []*prismgoclient.AdditionalFilter{{Name: "name", Values: []string{"keep"}}}
	for _, body := range []string{`{}`, `{"entities":null}`, `{"entities":{}}`,
		`{"entities":"invalid"}`, `{"entities":[null]}`, `{"entities":[42]}`} {
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
		_, err := filter(io.NopCloser(strings.NewReader(`{"entities":[]}`)),
			[]*prismgoclient.AdditionalFilter{nil}, nil)
		if err == nil {
			t.Fatal("expected nil filter error")
		}
	})
}

func TestHTTPErrorDoesNotExposeResponse(t *testing.T) {
	const marker = "fixture-secret-marker"
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
		response := &http.Response{StatusCode: http.StatusBadRequest,
			Status: marker, Body: io.NopCloser(strings.NewReader(body))}
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
	const jsonLimit = 64 << 20
	const errorLimit = 64 << 10
	largeJSON := `{"entities":[],"padding":"` + strings.Repeat("x", jsonLimit) + `"}`
	for _, tc := range []struct {
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
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.payload)
			body := &responseBody{Reader: reader}
			client := responseClient(body, tc.code)
			request, err := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if tc.filtered {
				err = client.DoWithFilters(t.Context(), request, &result,
					[]*prismgoclient.AdditionalFilter{{Name: "name", Values: []string{"keep"}}}, nil)
			} else {
				err = client.Do(t.Context(), request, &result)
			}
			var responseError *prismgoclient.ResponseError
			if !errors.Is(err, prismgoclient.ErrResponseTooLarge) || !errors.As(err, &responseError) || responseError.StatusCode != tc.code {
				t.Errorf("oversized response lost size/status classification: %v", err)
			}
			if read := len(tc.payload) - reader.Len(); read > tc.limit+1 {
				t.Errorf("body bytes read=%d, limit+1=%d", read, tc.limit+1)
			}
			if body.closes != 1 {
				t.Errorf("body Close calls=%d, want 1", body.closes)
			}
		})
	}
}

func TestResponseCancellation(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(fmt.Sprint(filtered), func(t *testing.T) {
			reading := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, `{"entities":[`)
				w.(http.Flusher).Flush()
				close(reading)
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			client := &Client{credentials: &prismgoclient.Credentials{}, httpClient: server.Client()}
			request, err := http.NewRequest(http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				var result map[string]any
				if filtered {
					done <- client.DoWithFilters(ctx, request, &result,
						[]*prismgoclient.AdditionalFilter{{Name: "name"}}, nil)
				} else {
					done <- client.Do(ctx, request, &result)
				}
			}()
			<-reading
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Errorf("cancellation cause lost: %v", err)
			}
		})
	}
}

func TestSafeDecodeErrorsAndStreaming(t *testing.T) {
	const marker = "fixture-secret-marker"
	for _, filtered := range []bool{false, true} {
		var result struct {
			Value string `json:"fixture-secret-marker"`
		}
		body := &responseBody{Reader: strings.NewReader(`{"fixture-secret-marker":123}`)}
		client := responseClient(body, http.StatusOK)
		request, err := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
		if err != nil {
			t.Fatal(err)
		}
		if filtered {
			err = client.DoWithFilters(t.Context(), request, &result, nil, nil)
		} else {
			err = client.Do(t.Context(), request, &result)
		}
		var typeError *json.UnmarshalTypeError
		if !errors.As(err, &typeError) || strings.Contains(fmt.Sprint(err), marker) {
			t.Errorf("unsafe decode error or lost cause: %v", err)
		}

		body = &responseBody{Reader: strings.NewReader(marker)}
		client = responseClient(body, http.StatusOK)
		var output strings.Builder
		if filtered {
			err = client.DoWithFilters(t.Context(), request, &output, nil, nil)
		} else {
			err = client.Do(t.Context(), request, &output)
		}
		if err != nil || output.String() != marker || body.closes != 1 {
			t.Errorf("stream changed: err=%v, data=%q, closes=%d", err, output.String(), body.closes)
		}
	}
	for _, size := range []int{8, 9} {
		_, err := readResponseBody(strings.NewReader(strings.Repeat("x", size)), 8)
		if (size == 8 && err != nil) || (size == 9 && !errors.Is(err, prismgoclient.ErrResponseTooLarge)) {
			t.Errorf("size %d: incorrect boundary error %v", size, err)
		}
	}
}
