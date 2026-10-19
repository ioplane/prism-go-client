package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/keploy/go-sdk/integrations/khttpclient"
	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testLibraryVersion = "v3"
	testAbsolutePath   = "api/nutanix/" + testLibraryVersion
	testUserAgent      = "nutanix/" + testLibraryVersion
	fileName           = "../v3/v3.go"
	testUsername       = "username"
	testPassword       = "password"
	testHost           = "foo.com"
	testFooPath        = "/foo"
	testName           = "name"
	testBar            = "bar"
	testContentType    = "Content-Type"
	testAccept         = "Accept"
	testAuthorization  = "Authorization"
	testKind           = "kind"
	testProject        = "project"
	testClientError    = "client error"
	testKey            = "key"
	testValue          = "value"
	testPath           = "/test"
	testUploadPath     = "/upload"
	testRootPath       = "/"
	testNilHTTPClient  = "nil httpClient"
	testHTTPMethods    = "different HTTP methods"
	testSessionID      = "session-id"
	testProjectsPath   = "/projects/list"
	testKeep           = "keep"
	testEmptyObject    = `{}`
	testBrokenJSON     = `{"broken":`
	testEmptyEntities  = `{"entities":[]}`
	testEndpoint       = "http://fixture.invalid"
	testSecretMarker   = "fixture-secret-marker"
)

func writeTestResponse(t *testing.T, writer http.ResponseWriter, payload string) {
	t.Helper()

	if _, err := io.WriteString(writer, payload); err != nil {
		t.Error(err)

		return
	}
}

func setup(t *testing.T) (*http.ServeMux, *Client, *httptest.Server) {
	t.Helper()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	creds := &prismgoclient.Credentials{URL: server.URL, Username: testUsername, Password: testPassword, Insecure: true}
	client := newTestClient(t, creds)

	return mux, client, server
}

func newTestClient(t *testing.T, credentials *prismgoclient.Credentials) *Client {
	t.Helper()

	client, err := NewClient(WithCredentials(credentials), WithUserAgent(testUserAgent),
		WithAbsolutePath(testAbsolutePath), WithBaseURL(credentials.URL))
	require.NoError(t, err)

	return client
}

func testClientOptions(credentials *prismgoclient.Credentials) []ClientOption {
	return []ClientOption{WithUserAgent(testUserAgent), WithAbsolutePath(testAbsolutePath), WithCredentials(credentials)}
}

type newClientTestCase struct {
	testCase    string
	opts        []ClientOption
	wantErr     bool
	expectedErr string
	validation  func(t *testing.T, client *Client)
}

func requiredClientCases() []newClientTestCase {
	return []newClientTestCase{
		{
			testCase: "valid",
			opts:     testClientOptions(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}),
		},
		{
			testCase: "no user agent",
			opts: []ClientOption{
				WithAbsolutePath(testAbsolutePath),
				WithCredentials(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}),
			},
			wantErr:     true,
			expectedErr: "userAgent argument must be passed",
		},
		{
			testCase: "no absolute path",
			opts: []ClientOption{
				WithUserAgent(testUserAgent),
				WithCredentials(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}),
			},
			wantErr:     true,
			expectedErr: "absolutePath argument must be passed",
		},
		{
			testCase: "no credentials",
			opts: []ClientOption{
				WithUserAgent(testUserAgent),
				WithAbsolutePath(testAbsolutePath),
			},
			wantErr:     true,
			expectedErr: "credentials argument must be passed",
		},
	}
}

func loggerURLClientCases(logger *logr.Logger) []newClientTestCase {
	return []newClientTestCase{
		{
			testCase: "custom logger",
			opts:     append(testClientOptions(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}), WithLogger(logger)),
			validation: func(t *testing.T, client *Client) {
				t.Helper()
				require.NotNil(t, client.logger)
				client.logger.Info("test")
			},
		},
		{
			testCase: "no logger creates default logger",
			opts:     testClientOptions(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}),
			validation: func(t *testing.T, client *Client) {
				t.Helper()
				assert.NotNil(t, client.logger)
			},
		},
		{
			testCase: "credentials URL is used when base URL is not provided",
			opts:     testClientOptions(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}),
			validation: func(t *testing.T, client *Client) {
				t.Helper()
				assert.Contains(t, client.BaseURL.String(), testHost)
			},
		},
		{
			testCase: "credentials URL is ignored when base URL is provided",
			opts: append(testClientOptions(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}),
				WithBaseURL("bar.com")),
			validation: func(t *testing.T, client *Client) {
				t.Helper()
				assert.Contains(t, client.BaseURL.String(), "bar.com")
				assert.NotContains(t, client.BaseURL.String(), testHost)
			},
		},
	}
}

func transportClientCases() []newClientTestCase {
	return []newClientTestCase{
		{
			testCase: "if insecure credentials is set, skip TLS verification",
			opts:     testClientOptions(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}),
			validation: func(t *testing.T, client *Client) {
				t.Helper()

				transport, ok := client.httpClient.Transport.(*http.Transport)
				require.True(t, ok)
				assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
			},
		},
		{
			testCase: "if insecure credentials is not set, ensure TLS verification",
			opts:     testClientOptions(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}),
			validation: func(t *testing.T, client *Client) {
				t.Helper()

				transport, ok := client.httpClient.Transport.(*http.Transport)
				require.True(t, ok)
				assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
			},
		},
		{
			testCase: "WithRoundTripper overrides the underlying transport",
			opts: append(testClientOptions(&prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword}),
				WithRoundTripper(khttpclient.NewInterceptor(http.DefaultTransport))),
			validation: func(t *testing.T, client *Client) {
				t.Helper()
				assert.IsType(t, &khttpclient.Interceptor{}, client.httpClient.Transport)
			},
		},
	}
}

func TestNewClient(t *testing.T) {
	// New returns a logr.Logger implemented by an arbitrary function.
	logger := testr.New(t)
	tests := append(requiredClientCases(), loggerURLClientCases(&logger)...)
	tests = append(tests, transportClientCases()...)

	for _, test := range tests {
		client, err := NewClient(test.opts...)
		if test.wantErr {
			assert.Nil(t, client)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedErr)
		} else {
			assert.NotNil(t, client)
			require.NoError(t, err)

			if test.validation != nil {
				test.validation(t, client)
			}
		}
	}
}

func TestNewRequest(t *testing.T) {
	creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: false}
	client := newTestClient(t, creds)

	inURL, outURL := testFooPath, fmt.Sprintf(defaultBaseURL+testAbsolutePath+testFooPath, "https", testHost)
	inBody, outBody := map[string]any{testName: testBar}, `{"name":"bar"}`+"\n"

	req, err := client.NewRequest(http.MethodPost, inURL, inBody)
	require.NoError(t, err)

	// test relative URL was expanded
	if req.URL.String() != outURL {
		t.Errorf("NewRequest(%v) URL = %v, expected %v", inURL, req.URL, outURL)
	}

	// test body was JSON encoded
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	if string(body) != outBody {
		t.Errorf("NewRequest(%v) Body = %v, expected %v", inBody, string(body), outBody)
	}
}

func TestNewUploadRequest(t *testing.T) {
	creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
	client := newTestClient(t, creds)

	inURL, outURL := testFooPath, fmt.Sprintf(defaultBaseURL+testAbsolutePath+testFooPath, SchemeHTTPS, testHost)
	inBody, err := os.Open(fileName)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, inBody.Close()) })

	// expected body
	out, err := os.Open(fileName)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, out.Close()) })

	outBody, err := io.ReadAll(out)
	require.NoError(t, err)

	req, err := client.NewUploadRequest(http.MethodPost, inURL, inBody)
	require.NoError(t, err)
	// test relative URL was expanded
	assert.Equal(t, outURL, req.URL.String())

	// test body contents
	got, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, outBody, got)

	// test headers.
	inHeaders := map[string]string{
		testContentType: octetStreamType,
		testAccept:      mediaType,
	}
	for k, v := range inHeaders {
		assert.Equal(t, v, req.Header[k][0])
	}
}

func TestNewUnAuthRequest(t *testing.T) {
	creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
	client := newTestClient(t, creds)

	inURL, outURL := testFooPath, fmt.Sprintf(defaultBaseURL+testAbsolutePath+testFooPath, SchemeHTTPS, testHost)
	inBody, outBody := map[string]any{testName: testBar}, `{"name":"bar"}`+"\n"

	req, err := client.NewUnAuthRequest(http.MethodPost, inURL, inBody)
	require.NoError(t, err)

	// test relative URL was expanded
	if req.URL.String() != outURL {
		t.Errorf("NewUnAuthRequest(%v) URL = %v, expected %v", inURL, req.URL, outURL)
	}

	// test body was JSON encoded
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	if string(body) != outBody {
		t.Errorf("NewUnAuthRequest(%v) Body = %v, expected %v", inBody, string(body), outBody)
	}

	// test headers. Authorization header shouldn't exist
	if _, ok := req.Header[testAuthorization]; ok {
		t.Error("Unexpected Authorization header obtained in request from NewUnAuthRequest()")
	}

	inHeaders := map[string]string{
		testContentType: mediaType,
		testAccept:      mediaType,
		"User-Agent":    testUserAgent,
	}
	for k, v := range req.Header {
		if v[0] != inHeaders[k] {
			t.Errorf("NewUnAuthRequest() Header value for %v = %v, expected %v", k, v[0], inHeaders[k])
		}
	}
}

func TestNewUnAuthFormEncodedRequest(t *testing.T) {
	creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
	client := newTestClient(t, creds)

	inURL, outURL := testFooPath, fmt.Sprintf(defaultBaseURL+testAbsolutePath+testFooPath, SchemeHTTPS, testHost)
	inBody := map[string]string{testName: testBar, "fullname": "foobar"}
	outBody := map[string][]string{testName: {testBar}, "fullname": {"foobar"}}

	req, err := client.NewUnAuthFormEncodedRequest(http.MethodPost, inURL, inBody)
	require.NoError(t, err)

	// test relative URL was expanded
	if req.URL.String() != outURL {
		t.Errorf("NewUnAuthFormEncodedRequest(%v) URL = %v, expected %v", inURL, req.URL, outURL)
	}

	// test body
	// Parse the body form data to a map structure which can be accessed by req.PostForm
	err = req.ParseForm()
	require.NoError(t, err)

	// check form encoded key-values as compared to input values
	if !reflect.DeepEqual(outBody, map[string][]string(req.PostForm)) {
		t.Errorf("NewUnAuthFormEncodedRequest(%v) Form encoded k-v, got = %v, expected %v", inBody, req.PostForm, outBody)
	}

	// test headers. Authorization header shouldn't exist
	if _, ok := req.Header[testAuthorization]; ok {
		t.Error("Unexpected Authorization header obtained in request from NewUnAuthFormEncodedRequest()")
	}

	inHeaders := map[string]string{
		testContentType: formEncodedType,
		testAccept:      mediaType,
		"User-Agent":    testUserAgent,
	}
	for k, v := range req.Header {
		if v[0] != inHeaders[k] {
			t.Errorf("NewUnAuthFormEncodedRequest() Header value for %v = %v, expected %v", k, v[0], inHeaders[k])
		}
	}
}

func TestNewUnAuthUploadRequest(t *testing.T) {
	creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
	client := newTestClient(t, creds)

	inURL, outURL := testFooPath, fmt.Sprintf(defaultBaseURL+testAbsolutePath+testFooPath, SchemeHTTPS, testHost)
	inBody, err := os.Open(fileName)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, inBody.Close()) })

	// expected body
	out, err := os.Open(fileName)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, out.Close()) })

	outBody, err := io.ReadAll(out)
	require.NoError(t, err)

	req, err := client.NewUnAuthUploadRequest(http.MethodPost, inURL, inBody)
	require.NoError(t, err)
	// test relative URL was expanded
	if req.URL.String() != outURL {
		t.Errorf("NewUnAuthUploadRequest(%v) URL = %v, expected %v", inURL, req.URL, outURL)
	}

	// test body contents
	got, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	if !bytes.Equal(got, outBody) {
		t.Errorf("NewUnAuthUploadRequest(%v) Body = %v, expected %v", inBody, string(got), string(outBody))
	}

	// test headers. Authorization header shouldn't exist
	if _, ok := req.Header[testAuthorization]; ok {
		t.Error("Unexpected Authorization header obtained in request from NewUnAuthUploadRequest()")
	}

	inHeaders := map[string]string{
		testContentType: octetStreamType,
		testAccept:      mediaType,
	}
	for k, v := range inHeaders {
		if v != req.Header[k][0] {
			t.Errorf("NewUploadRequest() Header value for %v = %v, expected %v", k, req.Header[k][0], v)
		}
	}
}

func TestErrorResponse_Error(t *testing.T) {
	messageResource := MessageResource{Message: "This field may not be blank."}
	messageList := make([]MessageResource, 1)
	messageList[0] = messageResource

	err := ErrorResponse{MessageList: messageList}

	if err.Error() == "" {
		t.Error("Expected non-empty ErrorResponse.Error()")
	}
}

func TestGetResponse(t *testing.T) {
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"api_version": "3.1", "code": 400, "kind": "error", "message_list":
				 [{"message": "This field may not be blank."}], "state": "ERROR"}`)),
	}

	err := CheckResponse(res)
	if err == nil {
		t.Fatal("Expected error response.")
	}

	var responseError *prismgoclient.ResponseError
	if !errors.As(err, &responseError) || responseError.StatusCode != http.StatusBadRequest {
		t.Errorf("error = %v, expected typed HTTP 400", err)
	}
}

func TestCheckResponse(t *testing.T) {
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"api_version": "3.1", "code": 400, "kind": "error", "message_list":
				 [{"message": "This field may not be blank."}], "state": "ERROR"}`)),
	}

	err := CheckResponse(res)
	if err == nil {
		t.Fatal("Expected error response.")
	}

	var responseError *prismgoclient.ResponseError
	if !errors.As(err, &responseError) || responseError.StatusCode != http.StatusBadRequest {
		t.Errorf("error = %v, expected typed HTTP 400", err)
	}
}

func TestDo(t *testing.T) {
	mux, client, server := setup(t)
	defer server.Close()

	type foo struct {
		A string
	}

	mux.HandleFunc(testRootPath, func(writer http.ResponseWriter, request *http.Request) {
		if m := http.MethodGet; m != request.Method {
			t.Errorf("Request method = %v, expected %v", request.Method, m)
		}

		writeTestResponse(t, writer, `{"A":"a"}`)
	})

	req, err := client.NewRequest(http.MethodGet, testRootPath, nil)
	require.NoError(t, err)

	body := new(foo)

	err = client.Do(context.Background(), req, body)
	if err != nil {
		t.Fatalf("Do(): %v", err)
	}

	expected := &foo{"a"}
	if !reflect.DeepEqual(body, expected) {
		t.Errorf("Response body = %v, expected %v", body, expected)
	}
}

func TestDo_httpError(t *testing.T) {
	mux, client, server := setup(t)
	defer server.Close()

	mux.HandleFunc(testRootPath, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "Bad Request", http.StatusBadRequest)
	})

	req, err := client.NewRequest(http.MethodGet, testRootPath, nil)
	require.NoError(t, err)

	err = client.Do(context.Background(), req, nil)
	if err == nil {
		t.Error("Expected HTTP 400 error.")
	}
}

// / Test handling of an error caused by the internal http httpClient's Do()
// function.
func TestDo_redirectLoop(t *testing.T) {
	mux, client, server := setup(t)
	defer server.Close()

	mux.HandleFunc(testRootPath, func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, testRootPath, http.StatusFound)
	})

	req, err := client.NewRequest(http.MethodGet, testRootPath, nil)
	require.NoError(t, err)

	err = client.Do(context.Background(), req, nil)
	if err == nil {
		t.Error("Expected error to be returned.")
	}

	var urlError *url.Error
	require.ErrorAs(t, err, &urlError)
}

func TestDo_SessionAuthRetryResetsBodyBeforeRetry(t *testing.T) {
	mux := http.NewServeMux()

	server := httptest.NewServer(mux)
	defer server.Close()

	var (
		projectCalls atomic.Int32
		usersMeCalls atomic.Int32
	)

	mux.HandleFunc("/api/nutanix/v3/users/me", func(writer http.ResponseWriter, _ *http.Request) {
		call := usersMeCalls.Add(1)
		writer.Header().Add("Set-Cookie", fmt.Sprintf("session-id=%d; Path=/", call))
		writer.WriteHeader(http.StatusOK)
		writeTestResponse(t, writer, testEmptyObject)
	})

	mux.HandleFunc("/api/nutanix/v3/projects/list", func(writer http.ResponseWriter, request *http.Request) {
		call := projectCalls.Add(1)

		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)

			return
		}

		assert.Len(t, body, 30)

		if call == 1 {
			writer.WriteHeader(http.StatusUnauthorized)
			writeTestResponse(t, writer, `{"message":"unauthorized"}`)

			return
		}

		writer.WriteHeader(http.StatusOK)
		writeTestResponse(t, writer, `{"metadata":{"total_matches":0,"offset":0},"entities":[]}`)
	})

	creds := &prismgoclient.Credentials{
		URL:         server.URL,
		Username:    testUsername,
		Password:    testPassword,
		Insecure:    true,
		SessionAuth: true,
	}
	client := newTestClient(t, creds)

	req, err := client.NewRequest(http.MethodPost, testProjectsPath, map[string]any{
		testKind: testProject,
		"length": 1,
	})
	require.NoError(t, err)

	var result map[string]any

	err = client.Do(context.Background(), req, &result)
	require.NoError(t, err)
	assert.Equal(t, int32(2), projectCalls.Load(), "second POST should reach server after body reset")
	assert.GreaterOrEqual(t, usersMeCalls.Load(), int32(2), "cookie refresh should occur before retry")
}

// func TestDo_completion_callback(t *testing.T) {
// 	setup()
// 	defer teardown()

// 	type foo struct {
// 		A string
// 	}

// 	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
// 		if m := http.MethodGet; m != r.Method {
// 			t.Errorf("Request method = %v, expected %v", r.Method, m)
// 		}
// 		_, _ = fmt.Fprint(w, `{"A":"a"}`)
// 	})

// 	req, _ := httpClient.NewRequest(ctx, http.MethodGet, "/", nil)
// 	req = req.WithContext(ctx)
// 	body := new(foo)

// 	// var completedReq *http.Request
// 	var completedResp string

// 	httpClient.OnRequestCompleted(func(req *http.Request, resp *http.Response, v interface{}) {
// 		// completedReq = req
// 		b, err := httputil.DumpResponse(resp, true)
// 		if err != nil {
// 			t.Errorf("Failed to dump response: %s", err)
// 		}
// 		completedResp = string(b)
// 	})
// 	err := httpClient.Do(ctx, req, body)

// 	if err != nil {
// 		t.Fatalf("Do(): %v", err)
// 	}

// 	// if !reflect.DeepEqual(req., completedReq) {
// 	// 	t.Errorf("Completed request = %v, expected %v", completedReq, req)
// 	// }

// 	expected := `{"A":"a"}`

// 	if !strings.Contains(completedResp, expected) {
// 		t.Errorf("expected response to contain %v, Response = %v", expected, completedResp)
// 	}
// }

// *********** Filters tests ***********

func getEntity(name, vlanID, uuid string) string {
	return fmt.Sprintf(`{"spec":{"cluster_reference":{"uuid":%q},"name":%q,"resources":{"vlan_id":%s}}}`, uuid, name, vlanID)
}

func removeWhiteSpace(input string) string {
	whitespacePattern := regexp.MustCompile(`\s+`)

	return whitespacePattern.ReplaceAllString(input, "")
}

func getFilter(name string, values []string) []*prismgoclient.AdditionalFilter {
	return []*prismgoclient.AdditionalFilter{
		{
			Name:   name,
			Values: values,
		},
	}
}

func runTest(t *testing.T, filters []*prismgoclient.AdditionalFilter, inputString, expected string) {
	t.Helper()

	input := io.NopCloser(strings.NewReader(inputString))
	baseSearchPaths := []string{"spec", "spec.resources"}
	filteredBody, err := filter(input, filters, baseSearchPaths)
	require.NoError(t, err)

	actualBytes, err := io.ReadAll(filteredBody)
	require.NoError(t, err)

	actual := string(actualBytes)
	assert.Equal(t, expected, actual)
}

func TestDoWithFilters_filter(t *testing.T) {
	entity1 := getEntity("subnet-01", "111", "012345-111")
	entity2 := getEntity("subnet-01", "112", "012345-112")
	entity3 := getEntity("subnet-02", "112", "012345-111")
	input := fmt.Sprintf(`{"entities":[%s,%s,%s]}`, entity1, entity2, entity3)

	filtersList := [][]*prismgoclient.AdditionalFilter{
		getFilter(testName, []string{"subnet-01", "subnet-03"}),
		getFilter("vlan_id", []string{"111", "subnet-03"}),
		getFilter("cluster_reference.uuid", []string{"111", "012345-112"}),
	}
	expectedList := []string{
		removeWhiteSpace(fmt.Sprintf(`{"entities":[%s,%s]}`, entity1, entity2)),
		removeWhiteSpace(fmt.Sprintf(`{"entities":[%s]}`, entity1)),
		removeWhiteSpace(fmt.Sprintf(`{"entities":[%s]}`, entity2)),
	}

	for i := range filtersList {
		runTest(t, filtersList[i], input, expectedList[i])
	}
}

func TestFilterDuplicateNames(t *testing.T) {
	input := `{"entities":[{"name":"a"},{"name":"b"},{"name":"c"}],"metadata":{"length":3}}`
	first := &prismgoclient.AdditionalFilter{Name: testName, Values: []string{"a", "b"}}
	tests := []struct {
		name     string
		second   *prismgoclient.AdditionalFilter
		expected string
	}{
		{
			name:     "identical predicates",
			second:   first,
			expected: `{"entities":[{"name":"a"},{"name":"b"}],"metadata":{"length":3}}`,
		},
		{
			name:     "intersecting values",
			second:   &prismgoclient.AdditionalFilter{Name: testName, Values: []string{"b", "c"}},
			expected: `{"entities":[{"name":"b"}],"metadata":{"length":3}}`,
		},
		{
			name:     "disjoint values",
			second:   &prismgoclient.AdditionalFilter{Name: testName, Values: []string{"c"}},
			expected: `{"entities":null,"metadata":{"length":3}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := filter(io.NopCloser(strings.NewReader(input)),
				[]*prismgoclient.AdditionalFilter{first, test.second}, []string{"$"})
			require.NoError(t, err)

			actual, err := io.ReadAll(body)
			require.NoError(t, err)
			require.NoError(t, body.Close())
			assert.JSONEq(t, test.expected, string(actual))
		})
	}
}

// Additional comprehensive tests for the specified methods

func TestNewUnAuthRequest_ErrorCases(t *testing.T) {
	t.Run(testNilHTTPClient, func(t *testing.T) {
		client := &Client{}
		client.ErrorMsg = testClientError

		req, err := client.NewUnAuthRequest(http.MethodGet, testPath, nil)
		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), testClientError)
	})

	t.Run(testHTTPMethods, func(t *testing.T) {
		creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
		client := newTestClient(t, creds)

		// Test different HTTP methods
		methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
		for _, method := range methods {
			req, err := client.NewUnAuthRequest(method, testPath, nil)
			require.NoError(t, err)
			assert.NotNil(t, req)
			assert.Equal(t, method, req.Method)
		}
	})

	t.Run("with body", func(t *testing.T) {
		creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
		client := newTestClient(t, creds)

		body := map[string]any{testKey: testValue}
		req, err := client.NewUnAuthRequest(http.MethodPost, testPath, body)
		require.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, "application/json", req.Header.Get(testContentType))
	})
}

func TestNewUnAuthFormEncodedRequest_ErrorCases(t *testing.T) {
	t.Run(testNilHTTPClient, func(t *testing.T) {
		client := &Client{}
		client.ErrorMsg = testClientError

		body := map[string]string{testKey: testValue}
		req, err := client.NewUnAuthFormEncodedRequest(http.MethodPost, testPath, body)
		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), testClientError)
	})

	t.Run(testHTTPMethods, func(t *testing.T) {
		creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
		client := newTestClient(t, creds)

		body := map[string]string{testKey: testValue}
		// Test different HTTP methods
		methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
		for _, method := range methods {
			req, err := client.NewUnAuthFormEncodedRequest(method, testPath, body)
			require.NoError(t, err)
			assert.NotNil(t, req)
			assert.Equal(t, method, req.Method)
		}
	})

	t.Run("empty form data", func(t *testing.T) {
		creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
		client := newTestClient(t, creds)

		body := map[string]string{}
		req, err := client.NewUnAuthFormEncodedRequest(http.MethodPost, testPath, body)
		require.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get(testContentType))
	})
}

func legacyUploadFile(t *testing.T, content string) *os.File {
	t.Helper()

	created, err := os.CreateTemp(t.TempDir(), "test-upload")
	require.NoError(t, err)

	if content != "" {
		_, err = created.WriteString(content)
		require.NoError(t, err)
	}

	require.NoError(t, created.Close())

	file, err := os.Open(created.Name())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })

	return file
}

func TestNewUploadRequest_ErrorCases(t *testing.T) {
	t.Run(testNilHTTPClient, func(t *testing.T) {
		client := &Client{}
		client.ErrorMsg = testClientError

		file := legacyUploadFile(t, "")

		req, err := client.NewUploadRequest(http.MethodPost, testUploadPath, file)
		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), testClientError)
	})

	t.Run(testHTTPMethods, func(t *testing.T) {
		creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
		client := newTestClient(t, creds)

		file := legacyUploadFile(t, "")

		// Test different HTTP methods
		methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch}
		for _, method := range methods {
			req, err := client.NewUploadRequest(method, testUploadPath, file)
			require.NoError(t, err)
			assert.NotNil(t, req)
			assert.Equal(t, method, req.Method)
		}
	})

	t.Run("file with content", func(t *testing.T) {
		creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
		client := newTestClient(t, creds)

		file := legacyUploadFile(t, "test content")

		req, err := client.NewUploadRequest(http.MethodPost, testUploadPath, file)
		require.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, "application/octet-stream", req.Header.Get(testContentType))
		assert.Contains(t, req.Header.Get(testAuthorization), "Basic")
	})
}

func TestNewUnAuthUploadRequest_ErrorCases(t *testing.T) {
	t.Run(testNilHTTPClient, func(t *testing.T) {
		client := &Client{}
		client.ErrorMsg = testClientError

		file := legacyUploadFile(t, "")

		req, err := client.NewUnAuthUploadRequest(http.MethodPost, testUploadPath, file)
		require.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), testClientError)
	})

	t.Run(testHTTPMethods, func(t *testing.T) {
		creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
		client := newTestClient(t, creds)

		file := legacyUploadFile(t, "")

		// Test different HTTP methods
		methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch}
		for _, method := range methods {
			req, err := client.NewUnAuthUploadRequest(method, testUploadPath, file)
			require.NoError(t, err)
			assert.NotNil(t, req)
			assert.Equal(t, method, req.Method)
		}
	})

	t.Run("file with content", func(t *testing.T) {
		creds := &prismgoclient.Credentials{URL: testHost, Username: testUsername, Password: testPassword, Insecure: true}
		client := newTestClient(t, creds)

		file := legacyUploadFile(t, "test content")

		req, err := client.NewUnAuthUploadRequest(http.MethodPost, testUploadPath, file)
		require.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, "application/octet-stream", req.Header.Get(testContentType))
		assert.Empty(t, req.Header.Get(testAuthorization))
	})
}

func TestDecorateRequestWithAuthHeaders_ServiceAccountModes(t *testing.T) {
	// 1) Explicit APIKey
	{
		creds := &prismgoclient.Credentials{
			APIKey: "explicit-api-key",
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", http.NoBody)
		require.NoError(t, err)
		decorateRequestWithAuthHeaders(req, creds)
		assert.Equal(t, "explicit-api-key", req.Header.Get(ntnxAPIKeyHeaderKey))
		assert.Empty(t, req.Header.Get(testAuthorization))
	}
	// 2) Username is API token header key, Password is APIKey
	{
		creds := &prismgoclient.Credentials{
			Username: ntnxAPIKeyHeaderKey,
			Password: "password-as-api-key",
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", http.NoBody)
		require.NoError(t, err)
		decorateRequestWithAuthHeaders(req, creds)
		assert.Equal(t, "password-as-api-key", req.Header.Get(ntnxAPIKeyHeaderKey))
		assert.Empty(t, req.Header.Get(testAuthorization))
	}
	// 3) Normal basic auth should remain unchanged
	{
		creds := &prismgoclient.Credentials{
			Username: testUsername,
			Password: testPassword,
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", http.NoBody)
		require.NoError(t, err)
		decorateRequestWithAuthHeaders(req, creds)
		assert.Empty(t, req.Header.Get(ntnxAPIKeyHeaderKey))
		auth := req.Header.Get(testAuthorization)
		assert.True(t, strings.HasPrefix(auth, "Basic "))
	}
}

type filterResponseFixture struct {
	name    string
	payload string
	status  int
	filters []*prismgoclient.AdditionalFilter
	paths   []string
}

func filterResponseFixtures() []filterResponseFixture {
	return []filterResponseFixture{
		{
			name: "successful request with filters", payload: `{
				"entities": [
					{"name": "entity1", "type": "type1"},
					{"name": "entity2", "type": "type2"}
				]
			}`, status: http.StatusOK,
			filters: []*prismgoclient.AdditionalFilter{{Name: "type", Values: []string{"type1"}}}, paths: []string{"$."},
		},
		{
			name: "request with error response", payload: `{
				"status": {
					"state": "ERROR",
					"message_list": [
						{"message": "Bad request", "reason": "invalid_input"}
					]
				}
			}`, status: http.StatusBadRequest,
			filters: nil, paths: nil,
		},
		{
			name: "request with nil filters", payload: `{
				"entities": [
					{"name": "entity1", "type": "type1"}
				]
			}`, status: http.StatusOK,
			filters: nil, paths: nil,
		},
		{
			name: "request with empty baseSearchPaths", payload: `{
				"entities": [
					{"name": "entity1", "type": "type1"}
				]
			}`, status: http.StatusOK,
			filters: []*prismgoclient.AdditionalFilter{{Name: "type", Values: []string{"type1"}}}, paths: []string{},
		},
	}
}

func TestDoWithFilters_ErrorCases(t *testing.T) {
	t.Run(testNilHTTPClient, func(t *testing.T) {
		client := &Client{}
		client.ErrorMsg = testClientError

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://test.com", http.NoBody)
		require.NoError(t, err)
		require.NoError(t, err)

		var result map[string]any

		err = client.DoWithFilters(context.Background(), req, &result, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), testClientError)
	})

	for _, fixture := range filterResponseFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			mux, client, server := setup(t)
			defer server.Close()

			mux.HandleFunc(testRootPath, func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(fixture.status)
				writeTestResponse(t, writer, fixture.payload)
			})
			req, err := client.NewRequest(http.MethodGet, testRootPath, nil)
			require.NoError(t, err)

			var result map[string]any

			err = client.DoWithFilters(context.Background(), req, &result, fixture.filters, fixture.paths)
			if fixture.status == http.StatusBadRequest {
				require.Error(t, err)

				var responseError *prismgoclient.ResponseError
				require.ErrorAs(t, err, &responseError)
				assert.Equal(t, http.StatusBadRequest, responseError.StatusCode)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
