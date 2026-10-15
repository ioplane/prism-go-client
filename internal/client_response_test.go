package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
				if err := CheckResponse(response); err == nil {
					t.Fatalf("HTTP %d accepted as success for body %q", code, body)
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

	closeError := errors.New("fixture close error")
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
