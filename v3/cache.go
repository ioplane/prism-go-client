package v3

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
)

type clientCacheMap map[string]*Client

type clientCreation struct {
	done chan struct{}
	hash string
	err  error
}

func (p *clientCreation) wait(ctx context.Context, hash string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		if p.hash == hash {
			return p.err
		}

		return nil
	}
}

// ClientCache is a cache for prism clients.
type ClientCache struct {
	cache            clientCacheMap
	validationHashes map[string]string
	creating         map[string]*clientCreation
	mtx              sync.RWMutex

	useSessionAuth bool
}

// CacheOpts is a functional option for the ClientCache.
type CacheOpts func(*ClientCache)

// WithSessionAuth sets the session auth for the ClientCache
// If sessionAuth is true, the client will use session auth instead of basic auth for authentication of requests
// If sessionAuth is false, the client will use basic auth for authentication of requests.
func WithSessionAuth(sessionAuth bool) CacheOpts {
	return func(c *ClientCache) {
		c.useSessionAuth = sessionAuth
	}
}

// NewClientCache returns a new ClientCache.
func NewClientCache(opts ...CacheOpts) *ClientCache {
	cache := &ClientCache{
		cache:            make(clientCacheMap),
		validationHashes: make(map[string]string),
		creating:         make(map[string]*clientCreation),
		mtx:              sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(cache)
	}

	return cache
}

// GetOrCreate uses Background; use GetOrCreateWithContext for cancelable creation.
// Options from the first successful publication win until endpoint change or Delete.
func (c *ClientCache) GetOrCreate(params types.CachedClientParams, opts ...types.ClientOption[Client]) (*Client, error) {
	return c.GetOrCreateWithContext(context.Background(), params, opts...)
}

// GetOrCreateWithContext serializes constructors per key, without holding the cache lock during I/O.
// Callers must keep params and option inputs immutable for the duration of the call.
func (c *ClientCache) GetOrCreateWithContext(ctx context.Context, params types.CachedClientParams, opts ...types.ClientOption[Client]) (*Client, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}

	key, endpoint := params.Key(), params.ManagementEndpoint()
	if err := validateManagementEndpoint(endpoint, key); err != nil {
		return nil, err
	}

	hash, err := endpoint.GetHash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate validation hash for cachedClientParams with key %s: %w", key, err)
	}

	return c.acquireClient(ctx, key, endpoint, hash, opts)
}

func (c *ClientCache) acquireClient(ctx context.Context, key string, endpoint types.ManagementEndpoint, hash string, opts []types.ClientOption[Client]) (*Client, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if client, cachedHash, _ := c.get(key); client != nil && cachedHash == hash {
			return client, nil
		}

		c.mtx.Lock()
		if client := c.cache[key]; client != nil && c.validationHashes[key] == hash {
			c.mtx.Unlock()

			return client, nil
		}

		pending := c.creating[key]
		if pending == nil {
			pending = &clientCreation{done: make(chan struct{}), hash: hash}
			c.creating[key] = pending
			c.mtx.Unlock()

			return c.create(ctx, key, endpoint, opts, pending)
		}
		c.mtx.Unlock()

		if err := pending.wait(ctx, hash); err != nil {
			return nil, err
		}
	}
}

func (c *ClientCache) create(ctx context.Context, key string, endpoint types.ManagementEndpoint, opts []types.ClientOption[Client], pending *clientCreation) (*Client, error) {
	client, err := c.newClient(ctx, key, endpoint, opts...)
	c.mtx.Lock()

	var retired *Client

	if c.creating[key] != pending {
		err = fmt.Errorf("cache entry invalidated during creation: %w", types.ErrorClientNotFound)
	} else {
		delete(c.creating, key)

		if err == nil {
			err = ctx.Err()
		}

		if err == nil {
			retired = c.cache[key]
			c.cache[key], c.validationHashes[key] = client, pending.hash
		}
	}

	pending.err = err
	close(pending.done)
	c.mtx.Unlock()

	if err != nil {
		client.closeIdleConnections()

		return nil, err
	}

	retired.closeIdleConnections()

	return client, nil
}

func (c *ClientCache) newClient(ctx context.Context, key string, managementEndpoint types.ManagementEndpoint, opts ...types.ClientOption[Client]) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	credentials := prismgoclient.Credentials{
		URL:         managementEndpoint.Address.Host,
		Endpoint:    managementEndpoint.Address.Host,
		Insecure:    managementEndpoint.Insecure,
		Username:    managementEndpoint.Username,
		Password:    managementEndpoint.Password,
		APIKey:      managementEndpoint.APIKey,
		SessionAuth: c.useSessionAuth,
	}

	setDefaultsForCredentials(&credentials)

	if err := validateCredentials(credentials, key); err != nil {
		return nil, fmt.Errorf("failed to validate credentials for cachedClientParams with key %s: %w", key, err)
	}

	if managementEndpoint.AdditionalTrustBundle != "" {
		opts = append(slices.Clone(opts), WithPEMEncodedCertBundle([]byte(managementEndpoint.AdditionalTrustBundle)))
	}

	client, err := NewV3ClientWithContext(ctx, credentials, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for cachedClientParams with key %s: %w", key, err)
	}

	return client, nil
}

func setDefaultsForCredentials(credentials *prismgoclient.Credentials) {
	if credentials.Port == "" {
		credentials.Port = "9440"
	}

	if credentials.URL == "" {
		credentials.URL = fmt.Sprintf("%s:%s", credentials.Endpoint, credentials.Port)
	}
}

func validateManagementEndpoint(endpoint types.ManagementEndpoint, key string) error {
	if endpoint.Address == nil {
		return fmt.Errorf("management endpoint address is nil for cachedClientParams with key %s", key)
	}

	// Defensive programming: validate required fields
	if endpoint.Address.Host == "" {
		return fmt.Errorf("management endpoint address host is empty for cachedClientParams with key %s", key)
	}

	return nil
}

func validateCredentials(credentials prismgoclient.Credentials, key string) error {
	if credentials.APIKey == "" && (credentials.Username == "" || credentials.Password == "") {
		return fmt.Errorf("API credentials(either a username and password, or an API key) is required for authentication"+
			" in cachedClientParams with key %s", key)
	}

	return nil
}

// Get returns the client and the validation hash for the given client name.
func (c *ClientCache) get(clientName string) (*Client, string, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	clnt, valueOK := c.cache[clientName]
	if !valueOK {
		return nil, "", types.ErrorClientNotFound
	}

	validationHash, valueOK := c.validationHashes[clientName]
	if !valueOK {
		return clnt, "", nil
	}

	return clnt, validationHash, nil
}

// Set adds the client to the cache.
func (c *ClientCache) set(clientName string, validationHash string, client *Client) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.cache[clientName] = client
	c.validationHashes[clientName] = validationHash
}

// Delete invalidates the cached entry and any pending publication for its key.
func (c *ClientCache) Delete(params types.CachedClientParams) {
	key := params.Key()

	c.mtx.Lock()
	retired := c.cache[key]
	delete(c.cache, key)
	delete(c.validationHashes, key)
	delete(c.creating, key)
	c.mtx.Unlock()
	retired.closeIdleConnections()
}

func (c *Client) closeIdleConnections() {
	if c != nil && c.client != nil {
		c.client.CloseIdleConnections()
	}
}
