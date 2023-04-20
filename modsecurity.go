// Package traefik_modsecurity_plugin is a plugin for the Traefik reverse proxy
// that integrates ModSecurity, a widely-used Web Application Firewall (WAF).
package traefik_modsecurity_plugin

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/patrickmn/go-cache"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// Config holds the plugin configuration.
type Config struct {
	TimeoutMillis  int64  `json:"timeoutMillis"`            // Timeout for HTTP requests to the ModSecurity server.
	ModSecurityUrl string `json:"modSecurityUrl,omitempty"` // The URL of the ModSecurity server.
	MaxBodySize    int64  `json:"maxBodySize"`              // Maximum allowed size of the request body.

	// Enable or disable cache globally.
	CacheEnabled *bool `json:"cacheEnabled,omitempty"`

	// CacheConditionsMethods specifies the HTTP methods for which caching is allowed.
	CacheConditionsMethods []string `json:"cacheConditionsMethods,omitempty"`

	// CacheConditionsNoBody specifies if requests with no body (content-length of 0) should be cached.
	CacheConditionsNoBody *bool `json:"cacheConditionsNoBody,omitempty"`

	// CacheKeyIncludeMethod specifies if the HTTP method should be included in the cache key.
	CacheKeyIncludeMethod *bool `json:"cacheKeyIncludeMethod,omitempty"`

	// CacheKeyIncludeRequestURI specifies if the request URI should be included in the cache key.
	CacheKeyIncludeRequestURI *bool `json:"cacheKeyIncludeRequestURI,omitempty"`

	// CacheKeyIncludeHeaders specifies if the headers should be included in the cache key.
	CacheKeyIncludeHeaders *bool `json:"cacheKeyIncludeHeaders,omitempty"`

	// CacheKeyHeaders lists the specific headers to be included in the cache key when CacheKeyIncludeHeaders is true.
	CacheKeyHeaders []string `json:"cacheKeyHeaders,omitempty"`

	// CacheKeyMatchAllHeaders specifies if all headers should be included in the cache key when CacheKeyIncludeHeaders is true.
	CacheKeyMatchAllHeaders *bool `json:"cacheKeyMatchAllHeaders,omitempty"`

	// CacheKeyIncludeHost specifies if the host should be included in the cache key.
	CacheKeyIncludeHost *bool `json:"cacheKeyIncludeHost,omitempty"`

	// CacheKeyIncludeRemoteAddress specifies if the remote address should be included in the cache key.
	CacheKeyIncludeRemoteAddress *bool `json:"cacheKeyIncludeRemoteAddress,omitempty"`

	// CacheConditions and CacheKey are structs to store parsed configurations for easy access.
	CacheConditions CacheKeyConditions
	CacheKey        CacheKeyOptions
}

// CreateConfig creates a default plugin configuration with predefined values.
func CreateConfig() *Config {

	init := Config{

		CacheEnabled: boolPtr(true),

		CacheConditionsMethods: []string{"GET"},
		CacheConditionsNoBody:  boolPtr(true),

		CacheKeyIncludeHost:          boolPtr(true),
		CacheKeyIncludeMethod:        boolPtr(true),
		CacheKeyIncludeRequestURI:    boolPtr(true),
		CacheKeyIncludeHeaders:       boolPtr(false),
		CacheKeyMatchAllHeaders:      boolPtr(false),
		CacheKeyHeaders:              []string{"Authorization", "User-Agent", "Cache-Control"},
		CacheKeyIncludeRemoteAddress: boolPtr(false),
	}

	finalize := Config{

		// The maximum amount of time (in milliseconds) to wait for a response from the backend server.
		TimeoutMillis: 2000,

		// The maximum size (in bytes) of the request body that the plugin will cache.
		// If the request body is larger than this, it will not be cached.
		MaxBodySize: 10 * 1024 * 1024,

		CacheEnabled: init.CacheEnabled,

		// Conditions that determine whether a request should be cached.
		// In this case, only GET requests with nobody (content-length of 0) will be cached.
		CacheConditions: CacheKeyConditions{
			Methods: init.CacheConditionsMethods,
			NoBody:  init.CacheConditionsNoBody,
		},

		// Options that determine how the cache key is generated.
		// In this case, the cache key will include the request method and URI, but not the headers, host, or body.
		CacheKey: CacheKeyOptions{
			IncludeMethod:        init.CacheKeyIncludeMethod,
			IncludeRequestURI:    init.CacheKeyIncludeRequestURI,
			IncludeHeaders:       init.CacheKeyIncludeHeaders,
			MatchAllHeaders:      init.CacheKeyMatchAllHeaders,
			Headers:              init.CacheKeyHeaders,
			IncludeHost:          init.CacheKeyIncludeHost,
			IncludeRemoteAddress: init.CacheKeyIncludeRemoteAddress,
		},
	}

	return &finalize

}

func boolPtr(b bool) *bool {
	return &b
}

type Modsecurity struct {
	next            http.Handler       // The next handler in the middleware chain.
	modSecurityUrl  string             // The URL of the ModSecurity server.
	maxBodySize     int64              // Maximum allowed size of the request body.
	name            string             // The plugin name.
	httpClient      *http.Client       // The HTTP client used to communicate with the ModSecurity server.
	logger          *log.Logger        // A logger for reporting events and errors.
	cache           *cache.Cache       // A cache for storing the results of requests that have already been processed.
	cacheEnabled    bool               // Enable or disable cache globally.
	cacheConditions CacheKeyConditions // rules for enabling cache
	cacheKey        CacheKeyOptions    // options for cache key generation
}

// New create a new Modsecurity plugin with the given configuration.
// It returns an HTTP handler that can be integrated into the Traefik middleware chain.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.ModSecurityUrl) == 0 {
		return nil, fmt.Errorf("modSecurityUrl cannot be empty")
	}

	// Use a custom client with predefined timeout of 2 seconds
	var timeout time.Duration
	if config.TimeoutMillis == 0 {
		timeout = 2 * time.Second
	} else {
		timeout = time.Duration(config.TimeoutMillis) * time.Millisecond
	}

	// dialer is a custom net.Dialer with a specified timeout and keep-alive duration.
	dialer := &net.Dialer{
		Timeout:   30 * time.Second, // Timeout is the maximum amount of time a dial will wait for a connect to complete.
		KeepAlive: 30 * time.Second, // KeepAlive specifies the interval between keep-alive probes for an active network connection.
	}

	// transport is a custom http.Transport with various timeouts and configurations for optimal performance.
	transport := &http.Transport{
		MaxIdleConns:          100,              // MaxIdleConns is the maximum number of idle connections across all hosts.
		IdleConnTimeout:       90 * time.Second, // IdleConnTimeout is the maximum amount of time an idle connection will remain idle before closing itself.
		TLSHandshakeTimeout:   10 * time.Second, // TLSHandshakeTimeout is the maximum time waiting to complete the TLS handshake.
		ExpectContinueTimeout: 1 * time.Second,  // ExpectContinueTimeout is the maximum time the Transport will wait for a server's first response headers after fully writing the request headers if the request has an "Expect: 100-continue" header.
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, // MinVersion contains the minimum SSL/TLS version that is acceptable. This example requires TLS 1.2 or higher.
		},
		ForceAttemptHTTP2: true, // ForceAttemptHTTP2 controls whether HTTP/2 is enabled when a non-zero DialContext is provided.
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) { // DialContext specifies the dial function for creating unencrypted TCP connections.
			return dialer.DialContext(ctx, network, addr)
		},
	}

	return &Modsecurity{
		modSecurityUrl:  config.ModSecurityUrl,
		maxBodySize:     config.MaxBodySize,
		next:            next,
		name:            name,
		httpClient:      &http.Client{Timeout: timeout, Transport: transport},
		logger:          log.New(os.Stdout, "", log.LstdFlags),
		cache:           cache.New(5*time.Minute, 10*time.Minute),
		cacheEnabled:    config.CacheEnabled != nil && *config.CacheEnabled,
		cacheConditions: config.CacheConditions,
		cacheKey:        config.CacheKey,
	}, nil
}

func (a *Modsecurity) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if isWebsocket(req) {
		a.next.ServeHTTP(rw, req)
		return
	}

	// a.logger.Printf("Request to modsec: method: %s, uri: %s, headers: %s, body: %s", req.Method, req.RequestURI, req.Header, req.Body)
	// a.logger.Printf("config.CacheConditionsMethods %v", a.cacheConditionsMethods)

	var resp *http.Response
	var respErr error
	var reqErr error

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		reqErr = a.HandleRequestBodyMaxSize(rw, req)
	}()

	go func() {
		defer wg.Done()
		resp, respErr = a.HandleCacheAndForwardRequest(req)
	}()

	wg.Wait()

	if reqErr != nil || respErr != nil {
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			a.logger.Printf("fail to close response body: %s", err.Error())
		}
	}(resp.Body)

	if resp.StatusCode >= 400 {
		forwardResponse(resp, rw)
		return
	}

	// a.logger.Printf("Response from modsec: status code: %d, headers: %v", resp.StatusCode, resp.Header)
	a.next.ServeHTTP(rw, req)
}

func (a *Modsecurity) PrepareForwardedRequest(req *http.Request) (*http.Response, error) {
	url := a.modSecurityUrl + req.RequestURI
	proxyReq, err := http.NewRequestWithContext(context.Background(), req.Method, url, req.Body)
	if err != nil {
		a.logger.Printf("fail to prepare forwarded request: %s", err.Error())
		return nil, err
	}

	proxyReq.Header = req.Header

	resp, err := a.httpClient.Do(proxyReq)
	if err != nil {
		a.logger.Printf("fail to send HTTP request to modsec: %s", err.Error())
		return nil, err
	}

	return resp, nil
}

// Forward the HTTP response to the client
func forwardResponse(resp *http.Response, rw http.ResponseWriter) {
	// copy other headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			rw.Header().Add(k, v)
		}
	}
	// copy status
	rw.WriteHeader(resp.StatusCode)
	// copy body
	io.Copy(rw, resp.Body)
}
