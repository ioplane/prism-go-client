// Package internal provides the shared Prism HTTP client implementation.
package internal

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/PaesslerAG/jsonpath"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/hashicorp/go-cleanhttp"
	"go.uber.org/zap"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
)

// Scheme identifies the HTTP URL scheme.
type Scheme string

const (
	defaultBaseURL         = "%s://%s/"
	mediaType              = "application/json"
	formEncodedType        = "application/x-www-form-urlencoded"
	octetStreamType        = "application/octet-stream"
	ntnxAPIKeyHeaderKey    = "X-Ntnx-Api-Key"
	maxJSONResponseBytes   = 64 << 20
	maxErrorResponseBytes  = 64 << 10
	requestURLParseMessage = "error parsing request URL"

	// SchemeHTTP is the unencrypted HTTP URL scheme.
	SchemeHTTP Scheme = "http"
	// SchemeHTTPS is the TLS-protected HTTP URL scheme.
	SchemeHTTPS Scheme = "https"
)

// RequestCompletionCallback defines the type of the request callback function.
type RequestCompletionCallback func(*http.Request, *http.Response, any)

// Client Config Configuration of the httpClient.
type Client struct {
	credentials *prismgoclient.Credentials

	// HTTP httpClient used to communicate with the Nutanix API.
	httpClient     *http.Client
	ownedTransport *http.Transport

	// Base URL for API requests.
	BaseURL *url.URL

	// User agent for httpClient
	UserAgent string

	cookies        []*http.Cookie
	sessionMu      sync.Mutex
	sessionVersion uint64
	sessionRefresh *cookieRefresh

	// Optional function called after every successful request made.
	onRequestCompleted RequestCompletionCallback

	// absolutePath: for example api/nutanix/v3
	absolutePath string

	// error message, incase httpClient is in error state
	ErrorMsg string

	logger   *logr.Logger
	certpool *x509.CertPool
}

type cookieRefresh struct {
	done chan struct{}
	err  error
}

// ClientOption configures a shared HTTP client.
type ClientOption func(*Client) error

type urlDiagnosticError struct {
	message string
	cause   error
}

// The original URL error may contain credentials; only Unwrap exposes it.
func (err *urlDiagnosticError) Error() string { return err.message }

func (err *urlDiagnosticError) Unwrap() error { return err.cause }

func (err *urlDiagnosticError) Timeout() bool {
	cause, ok := err.cause.(interface{ Timeout() bool })

	return ok && cause.Timeout()
}

func (err *urlDiagnosticError) Temporary() bool {
	cause, ok := err.cause.(interface{ Temporary() bool })

	return ok && cause.Temporary()
}

// WithLogger sets the logger for the httpClient.
func WithLogger(logger *logr.Logger) ClientOption {
	return func(client *Client) error {
		if logger == nil {
			return errors.New("logger must not be nil")
		}

		client.logger = logger

		return nil
	}
}

// WithCredentials sets the credentials for the httpClient.
func WithCredentials(credentials *prismgoclient.Credentials) ClientOption {
	return func(client *Client) error {
		if credentials == nil {
			return errors.New("credentials must not be nil")
		}

		client.credentials = credentials
		if client.credentials.Insecure {
			transport, ok := client.httpClient.Transport.(*http.Transport)
			if !ok {
				return fmt.Errorf("transport is not of type http.Transport: %T", client.httpClient.Transport)
			}

			transport.TLSClientConfig.InsecureSkipVerify = true
		}

		if client.credentials.ProxyURL != "" {
			client.logger.V(1).Info("Using configured proxy")

			proxy, err := url.Parse(client.credentials.ProxyURL)
			if err != nil {
				return &urlDiagnosticError{message: "error parsing proxy URL", cause: err}
			}

			transport, ok := client.httpClient.Transport.(*http.Transport)
			if !ok {
				return fmt.Errorf("transport is not of type http.Transport: %T", client.httpClient.Transport)
			}

			transport.Proxy = http.ProxyURL(proxy)
		}

		return nil
	}
}

// WithUserAgent sets the user agent for the httpClient.
func WithUserAgent(userAgent string) ClientOption {
	return func(client *Client) error {
		client.UserAgent = userAgent

		return nil
	}
}

// WithBaseURL sets the base URL for the httpClient to communicate with.
func WithBaseURL(baseURL string) ClientOption {
	return func(client *Client) error {
		// if the BaseURL does not have a scheme, use https as a default scheme
		// the `url.Parse` function parses the base URL (i.e. Host) as a Path
		// if the URL does not have a scheme. Prefixing a scheme ensures the base URL
		// is parsed as a Host and not a Path.
		if !strings.HasPrefix(baseURL, string(SchemeHTTP)) {
			baseURL = fmt.Sprintf("%s://%s", SchemeHTTPS, baseURL)
		}

		requestURL, err := url.Parse(baseURL)
		if err != nil {
			return &urlDiagnosticError{message: "error parsing base URL", cause: err}
		}

		if requestURL.Path == "" {
			requestURL.Path = "/"
		}

		client.BaseURL = requestURL

		return nil
	}
}

// WithCookies sets the cookies for the httpClient.
func WithCookies(cookies []*http.Cookie) ClientOption {
	return func(client *Client) error {
		client.sessionMu.Lock()
		client.cookies = cookies
		client.sessionVersion++
		client.sessionMu.Unlock()

		return nil
	}
}

// WithAbsolutePath sets the absolute path for the httpClient to communicate with.
func WithAbsolutePath(absolutePath string) ClientOption {
	return func(client *Client) error {
		client.absolutePath = absolutePath

		return nil
	}
}

// WithCertificate adds the certificate to the certificate pool in tls config.
func WithCertificate(cert *x509.Certificate) ClientOption {
	return func(client *Client) error {
		if cert == nil {
			return errors.New("certificate is nil")
		}

		client.certpool.AddCert(cert)

		return nil
	}
}

// WithRoundTripper overrides the transport for the httpClient
// Overriding transport is useful for testing against API Mocks
// This is not recommended for production use
// A nil interface restores the client's private default transport.
func WithRoundTripper(transport http.RoundTripper) ClientOption {
	return func(client *Client) error {
		if transport == nil {
			client.httpClient.Transport = client.ownedTransport

			return nil
		}

		if concrete, ok := transport.(*http.Transport); ok && concrete == nil {
			return errors.New("HTTP transport must not be nil")
		}

		client.httpClient.Transport = transport

		return nil
	}
}

// NewClient returns a wrapper around http/https (as per isHTTP flag) httpClient with additions of proxy & session_auth if given.
func NewClient(opts ...ClientOption) (*Client, error) {
	return NewClientWithContext(context.Background(), opts...)
}

// NewClientWithContext uses ctx for constructor authentication only.
func NewClientWithContext(ctx context.Context, opts ...ClientOption) (*Client, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client := &Client{}
	client.httpClient = cleanhttp.DefaultClient()

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to get system cert pool: %w", err)
	}

	client.certpool = certPool

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok || defaultTransport == nil {
		return nil, fmt.Errorf("default transport is not a non-nil *http.Transport: %T", http.DefaultTransport)
	}
	// Credentials and certificate options must never mutate the shared default.
	transport := defaultTransport.Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: client.certpool}
	client.httpClient.Transport = transport
	client.ownedTransport = transport

	// If the user does not specify a logger, then we'll use zap for a default one
	// If the user specified a logger, then we'll use that logger
	zapLog, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	logger := zapr.NewLogger(zapLog)
	client.logger = &logger

	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("client option must not be nil")
		}

		if err := opt(client); err != nil {
			return nil, err
		}
	}

	if err := ctx.Err(); err != nil {
		client.CloseIdleConnections()

		return nil, err
	}

	if err := client.validateOptions(); err != nil {
		return nil, err
	}

	if !client.credentials.SessionAuth {
		return client, nil
	}

	client.logger.V(1).Info("Using session_auth")

	if err := client.refreshCookies(ctx); err != nil {
		client.CloseIdleConnections()

		return nil, err
	}

	return client, nil
}

// CloseIdleConnections closes only the transport allocated by this client.
func (c *Client) CloseIdleConnections() {
	if c.ownedTransport != nil {
		c.ownedTransport.CloseIdleConnections()
	}
}

func decorateRequestWithAuthHeaders(req *http.Request, credentials *prismgoclient.Credentials) {
	if credentials.APIKey != "" {
		decorateRequestWithAPIKeyHeaders(req, credentials.APIKey)

		return
	}
	// If username equals the API token header key and password is provided,
	// treat password as API key and set header accordingly.
	if strings.EqualFold(credentials.Username, ntnxAPIKeyHeaderKey) && credentials.Password != "" {
		decorateRequestWithAPIKeyHeaders(req, credentials.Password)

		return
	}

	decorateRequestWithBasicAuthHeaders(req, credentials.Username, credentials.Password)
}

// NewRequest creates a request.
func (c *Client) NewRequest(method, urlStr string, body any) (*http.Request, error) {
	req, err := c.NewUnAuthRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}

	c.sessionMu.Lock()
	if c.cookies != nil {
		decorateRequestWithCookies(req, c.cookies)
	} else {
		decorateRequestWithAuthHeaders(req, c.credentials)
	}
	c.sessionMu.Unlock()

	return req, nil
}

// decorateRequestWithAPIKeyHeaders adds the API key to the request header.
func decorateRequestWithAPIKeyHeaders(req *http.Request, apiKey string) {
	req.Header.Add(ntnxAPIKeyHeaderKey, apiKey)
}

func decorateRequestWithBasicAuthHeaders(req *http.Request, username, password string) {
	req.Header.Add("Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}

func decorateRequestWithCookies(req *http.Request, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
}

func clearCookiesInRequest(req *http.Request) {
	req.Header.Del("Cookie")
}

// NewUnAuthRequest creates a request without authorisation headers.
func (c *Client) NewUnAuthRequest(method, urlStr string, body any) (*http.Request, error) {
	// check if httpClient exists or not
	if c.httpClient == nil {
		return nil, errors.New(c.ErrorMsg)
	}

	// create main api url
	rel, err := url.Parse(c.absolutePath + urlStr)
	if err != nil {
		return nil, &urlDiagnosticError{message: requestURLParseMessage, cause: err}
	}

	requestURL := c.BaseURL.ResolveReference(rel)

	buf := new(bytes.Buffer)
	if body != nil {
		err = json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(context.Background(), method, requestURL.String(), buf)
	if err != nil {
		return nil, err
	}

	// add api headers
	req.Header.Add("Content-Type", mediaType)
	req.Header.Add("Accept", mediaType)
	req.Header.Add("User-Agent", c.UserAgent)

	return req, nil
}

// NewUnAuthFormEncodedRequest returns content-type: application/x-www-form-urlencoded based unauth request.
func (c *Client) NewUnAuthFormEncodedRequest(method, urlStr string, body map[string]string) (*http.Request, error) {
	// check if httpClient exists or not
	if c.httpClient == nil {
		return nil, errors.New(c.ErrorMsg)
	}
	// create main api url
	rel, err := url.Parse(c.absolutePath + urlStr)
	if err != nil {
		return nil, &urlDiagnosticError{message: requestURLParseMessage, cause: err}
	}

	requestURL := c.BaseURL.ResolveReference(rel)

	// create form data from body
	data := url.Values{}
	for k, v := range body {
		data.Set(k, v)
	}

	// create a new request based on encoded from data
	req, err := http.NewRequestWithContext(context.Background(), method, requestURL.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// add api headers
	req.Header.Add("Content-Type", formEncodedType)
	req.Header.Add("Accept", mediaType)
	req.Header.Add("User-Agent", c.UserAgent)

	return req, nil
}

// NewUploadRequest authenticates an upload built by NewUnAuthUploadRequest.
func (c *Client) NewUploadRequest(method, urlStr string, fileReader *os.File) (*http.Request, error) {
	req, err := c.NewUnAuthUploadRequest(method, urlStr, fileReader)
	if err != nil {
		return nil, err
	}

	decorateRequestWithAuthHeaders(req, c.credentials)

	return req, nil
}

// NewUnAuthUploadRequest reads a regular file from its current offset without owning it.
// The caller must keep the file open and unchanged until the request and all replays finish.
func (c *Client) NewUnAuthUploadRequest(method, urlStr string, fileReader *os.File) (*http.Request, error) {
	// check if httpClient exists or not
	if c.httpClient == nil {
		return nil, errors.New(c.ErrorMsg)
	}

	rel, err := url.Parse(c.absolutePath + urlStr)
	if err != nil {
		return nil, &urlDiagnosticError{message: requestURLParseMessage, cause: err}
	}

	requestURL := c.BaseURL.ResolveReference(rel)

	fileInfo, err := fileReader.Stat()
	if err != nil {
		return nil, err
	}

	if !fileInfo.Mode().IsRegular() {
		return nil, errors.New("upload requires a regular file")
	}

	offset, err := fileReader.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	size := max(int64(0), fileInfo.Size()-offset)
	newBody := func() io.ReadCloser {
		if size == 0 {
			return http.NoBody
		}

		return io.NopCloser(io.NewSectionReader(fileReader, offset, size))
	}

	req, err := http.NewRequestWithContext(context.Background(), method, requestURL.String(), newBody())
	if err != nil {
		return nil, err
	}

	req.ContentLength = size
	req.GetBody = func() (io.ReadCloser, error) { return newBody(), nil }

	req.Header.Add("Content-Type", octetStreamType)
	req.Header.Add("Accept", mediaType)
	req.Header.Add("User-Agent", c.UserAgent)

	return req, nil
}

// OnRequestCompleted sets the DO API request completion callback.
func (c *Client) OnRequestCompleted(rc RequestCompletionCallback) {
	c.onRequestCompleted = rc
}

// Do performs the request with at most one session-auth replay.
func (c *Client) Do(ctx context.Context, req *http.Request, result any) error {
	req = req.Clone(ctx)

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) { _ = body.Close() }(resp.Body)

	if responseErr := CheckResponse(resp); responseErr != nil {
		return responseErr
	}

	if result != nil {
		if w, ok := result.(io.Writer); ok {
			_, err = io.Copy(w, resp.Body)
		} else {
			err = decodeResponse(resp.Body, result)
		}

		if err != nil {
			return &prismgoclient.ResponseError{StatusCode: resp.StatusCode, Cause: err}
		}
	}

	if c.onRequestCompleted != nil {
		c.onRequestCompleted(req, resp, result)
	}

	return nil
}

// DoWithFilters performs request passed and filters entities in json response.
func (c *Client) DoWithFilters(
	ctx context.Context, req *http.Request, result any,
	filters []*prismgoclient.AdditionalFilter, baseSearchPaths []string,
) (err error) {
	// check if httpClient exists or not
	if c.httpClient == nil {
		return errors.New(c.ErrorMsg)
	}

	req = req.Clone(ctx)

	resp, err := c.send(req)
	if err != nil {
		return err
	}

	// Filtering and CheckResponse may replace resp.Body; retain its original owner.
	defer func(body io.ReadCloser) {
		if rerr := body.Close(); err == nil && rerr != nil {
			err = &prismgoclient.ResponseError{StatusCode: resp.StatusCode, Cause: rerr}
		}
	}(resp.Body)

	if responseErr := CheckResponse(resp); responseErr != nil {
		return responseErr
	}

	resp.Body, err = filter(resp.Body, filters, baseSearchPaths)
	if err != nil {
		return &prismgoclient.ResponseError{StatusCode: resp.StatusCode, Cause: err}
	}

	if result != nil {
		if w, ok := result.(io.Writer); ok {
			_, err = io.Copy(w, resp.Body)
		} else if len(filters) == 0 {
			err = decodeResponse(resp.Body, result)
		} else {
			// Filtering already bounded the network body before JSON re-encoding.
			err = json.NewDecoder(resp.Body).Decode(result)
		}

		if err != nil {
			return &prismgoclient.ResponseError{StatusCode: resp.StatusCode, Cause: err}
		}
	}

	if c.onRequestCompleted != nil {
		c.onRequestCompleted(req, resp, result)
	}

	return nil
}

func filter(body io.ReadCloser, filters []*prismgoclient.AdditionalFilter, baseSearchPaths []string) (io.ReadCloser, error) {
	if len(filters) == 0 {
		return body, nil
	}

	if len(baseSearchPaths) == 0 {
		baseSearchPaths = []string{"$."}
	}

	var res map[string]any

	data, err := readResponseBody(body, maxJSONResponseBytes)
	if err != nil {
		return body, err
	}

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}

	// Full search paths
	searchPaths := make([][]string, len(filters))

	for index, filter := range filters {
		if filter == nil {
			return nil, errors.New("additional filter must not be nil")
		}

		// Build search paths by appending target search paths to base paths
		filterSearchPaths := []string{}

		for _, baseSearchPath := range baseSearchPaths {
			searchPath := fmt.Sprintf("%s.%s", baseSearchPath, filter.Name)
			filterSearchPaths = append(filterSearchPaths, searchPath)
		}

		searchPaths[index] = filterSearchPaths
	}

	filteredEntities, err := filterEntities(res["entities"], filters, searchPaths)
	if err != nil {
		return nil, err
	}

	res["entities"] = filteredEntities

	// Convert filtered result back to io.ReadCloser
	filteredBody, jsonErr := json.Marshal(res)
	if jsonErr != nil {
		return body, jsonErr
	}

	return io.NopCloser(bytes.NewReader(filteredBody)), nil
}

// filterEntities validates entity shapes and preserves source order and null for no matches.
func filterEntities(rawEntities any, filters []*prismgoclient.AdditionalFilter, searchPaths [][]string) ([]any, error) {
	entities, ok := rawEntities.([]any)
	if !ok {
		return nil, errors.New("filtered response entities must be an array")
	}

	var filteredEntities []any

	for _, entity := range entities {
		searchTarget, ok := entity.(map[string]any)
		if !ok {
			return nil, errors.New("filtered response entity must be an object")
		}

		filtersPassed := 0

		for index, predicate := range filters {
			if matchesFilter(searchTarget, predicate.Values, searchPaths[index]) {
				filtersPassed++
			}
		}

		// Each original predicate participates in AND, including duplicate names.
		if filtersPassed == len(filters) {
			filteredEntities = append(filteredEntities, entity)
		}
	}

	return filteredEntities, nil
}

// matchesFilter combines one predicate's paths and string values with OR.
func matchesFilter(searchTarget map[string]any, values, searchPaths []string) bool {
	for _, searchPath := range searchPaths {
		value, err := jsonpath.Get(searchPath, searchTarget)
		if err != nil {
			continue
		}

		// Leaf values retain the legacy string representation.
		if slices.Contains(values, fmt.Sprint(value)) {
			return true
		}
	}

	return false
}

func readResponseBody(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, prismgoclient.ErrResponseTooLarge
	}

	return data, nil
}

func decodeResponse(body io.Reader, result any) error {
	data, err := readResponseBody(body, maxJSONResponseBytes)
	if err != nil {
		return err
	}

	return json.NewDecoder(bytes.NewReader(data)).Decode(result)
}

// CheckResponse rejects non-2xx status codes without exposing server diagnostics.
func CheckResponse(response *http.Response) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	responseError := &prismgoclient.ResponseError{StatusCode: response.StatusCode, Cause: nil}
	if response.StatusCode == http.StatusUnauthorized {
		return responseError
	}

	buf, err := readResponseBody(response.Body, maxErrorResponseBytes)
	if err != nil {
		responseError.Cause = err

		return responseError
	}
	// Preserve bounded error-body replay; the caller still closes the original body.
	response.Body = io.NopCloser(bytes.NewReader(buf))

	return responseError
}

func (c *Client) validateOptions() error {
	if c.UserAgent == "" {
		return errors.New("userAgent argument must be passed")
	}

	if c.absolutePath == "" {
		return errors.New("absolutePath argument must be passed")
	}

	if c.credentials == nil {
		return errors.New("credentials argument must be passed")
	}

	if c.BaseURL == nil {
		c.logger.V(1).Info("BaseURL is not set. Using URL from credentials")

		if err := WithBaseURL(c.credentials.URL)(c); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) refreshCookies(ctx context.Context) error {
	req, err := c.NewUnAuthRequest(http.MethodGet, "/users/me", nil)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	decorateRequestWithAuthHeaders(req, c.credentials)

	resp, err := c.doHTTPRequest(req)
	if err != nil {
		return err
	}

	defer func(body io.ReadCloser) { _ = body.Close() }(resp.Body)

	if responseErr := CheckResponse(resp); responseErr != nil {
		return responseErr
	}

	c.sessionMu.Lock()
	c.cookies = resp.Cookies()
	c.sessionVersion++
	c.sessionMu.Unlock()

	return nil
}

// refreshSession shares one refresh result among callers using the expired version.
func (c *Client) refreshSession(req *http.Request, version uint64) error {
	ctx := req.Context()

	c.sessionMu.Lock()
	if version != c.sessionVersion {
		c.sessionMu.Unlock()

		return nil
	}
	// Requests built before a completed refresh may still carry older cookies.
	for _, cookie := range c.cookies {
		if sent, err := req.Cookie(cookie.Name); err == nil && sent.Value != cookie.Value {
			c.sessionMu.Unlock()

			return nil
		}
	}

	if refresh := c.sessionRefresh; refresh != nil {
		c.sessionMu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-refresh.done:
			return refresh.err
		}
	}

	refresh := &cookieRefresh{done: make(chan struct{}), err: nil}
	c.sessionRefresh = refresh
	c.sessionMu.Unlock()

	err := c.refreshCookies(ctx)
	c.sessionMu.Lock()
	refresh.err = err
	c.sessionRefresh = nil

	close(refresh.done)
	c.sessionMu.Unlock()

	return err
}

// doHTTPRequest hides URL diagnostics while retaining response and body ownership.
func (c *Client) doHTTPRequest(req *http.Request) (*http.Response, error) {
	response, err := c.httpClient.Do(req)
	if err != nil {
		return response, &urlDiagnosticError{message: "error sending HTTP request", cause: err}
	}

	return response, nil
}

// send permits one explicit 401 replay; transport errors never trigger a replay.
func (c *Client) send(req *http.Request) (*http.Response, error) {
	var version uint64

	if c.credentials.SessionAuth {
		c.sessionMu.Lock()
		version = c.sessionVersion
		c.sessionMu.Unlock()
	}

	resp, err := c.doHTTPRequest(req)
	if err != nil || !c.credentials.SessionAuth || resp.StatusCode != http.StatusUnauthorized ||
		(req.Body != nil && req.Body != http.NoBody && req.GetBody == nil) {
		return resp, err
	}

	_ = resp.Body.Close()

	if refreshErr := c.refreshSession(req, version); refreshErr != nil {
		return nil, refreshErr
	}

	if req.Body != nil && req.Body != http.NoBody {
		req.Body, err = req.GetBody()
		if err != nil {
			return nil, err
		}
	}

	c.sessionMu.Lock()
	clearCookiesInRequest(req)
	decorateRequestWithCookies(req, c.cookies)
	c.sessionMu.Unlock()

	return c.doHTTPRequest(req)
}
