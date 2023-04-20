package traefik_modsecurity_plugin

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
)

// CacheKeyConditions contains the conditions for generating a cache key.
type CacheKeyConditions struct {
	Methods []string
	NoBody  *bool
}

// CacheKeyOptions contains the options for generating a cache key.
type CacheKeyOptions struct {
	IncludeMethod        *bool
	IncludeRequestURI    *bool
	IncludeHeaders       *bool
	Headers              []string
	MatchAllHeaders      *bool
	IncludeHost          *bool
	IncludeRemoteAddress *bool
}

// Check evaluates if the current request meets the caching conditions.
func (c *CacheKeyConditions) Check(req *http.Request) bool {
	if len(c.Methods) > 0 && !contains(c.Methods, req.Method) {
		return false
	}
	if c.NoBody != nil && *c.NoBody != (req.ContentLength == 0) {
		return false
	}
	return true
}

var cacheResponsePool = sync.Pool{
	New: func() interface{} {
		return &http.Response{
			Header: make(http.Header),
		}
	},
}

func newPooledCacheResponse(statusCode int, body []byte) *http.Response {
	resp := cacheResponsePool.Get().(*http.Response)
	resp.StatusCode = statusCode
	resp.Body = ioutil.NopCloser(bytes.NewReader(body))

	// Set the status text based on the body
	statusText := http.StatusText(statusCode)
	if len(body) > 0 {
		statusText = string(body)
	}
	// Set the status text
	resp.Header.Set("Status-Text", statusText)
	return resp
}

func putResponse(resp *http.Response) {
	resp.Body.Close()
	resp.Header = make(http.Header)
	cacheResponsePool.Put(resp)
}

// generateCacheKey creates a cache key based on the request and options provided.
func generateCacheKey(req *http.Request, options CacheKeyOptions) string {
	hasher := sha256.New()

	if options.IncludeMethod != nil && *options.IncludeMethod {
		hasher.Write([]byte(req.Method))
	}

	if options.IncludeRequestURI != nil && *options.IncludeRequestURI {
		cleanedURL := removeTrackingParams(req.RequestURI)
		hasher.Write([]byte(cleanedURL))
	}

	if options.IncludeHeaders != nil && *options.IncludeHeaders {
		if options.MatchAllHeaders != nil && *options.MatchAllHeaders {
			for key, values := range req.Header {
				hasher.Write([]byte(key))
				for _, value := range values {
					hasher.Write([]byte(value))
				}
			}
		} else {
			headerKeys := make(map[string]struct{}, len(options.Headers))
			for _, key := range options.Headers {
				headerKeys[key] = struct{}{}
			}

			for key, values := range req.Header {
				if _, ok := headerKeys[key]; ok {
					hasher.Write([]byte(key))
					for _, value := range values {
						hasher.Write([]byte(value))
					}
				}
			}
		}
	}

	if options.IncludeHost != nil && *options.IncludeHost {
		hasher.Write([]byte(req.Host))
	}

	if options.IncludeRemoteAddress != nil && *options.IncludeRemoteAddress {
		hasher.Write([]byte(req.RemoteAddr))
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func (a *Modsecurity) GetCachedResponse(req *http.Request, options CacheKeyOptions) (*http.Response, error) {
	cacheKey := generateCacheKey(req, options)

	if cachedErrorCode, found := a.cache.Get(cacheKey); found {
		cachedCode := cachedErrorCode.(int)

		// convert cachedCode to a 3-digit code
		statusCode := cachedCode % 1000

		body := []byte(http.StatusText(statusCode))

		// create a new *http.Response with the given status code and body using the response pool
		resp := newPooledCacheResponse(statusCode, body)

		// Defer the call to putResponse to return the response object to the pool when done
		defer putResponse(resp)

		return resp, nil
	}

	resp, err := a.PrepareForwardedRequest(req)
	if err != nil {
		return nil, err
	}

	// only cache response codes
	cachedCode := resp.StatusCode % 1000
	a.cache.SetDefault(cacheKey, cachedCode)

	// send modsecurity response back up the chain to the client
	return resp, nil
}
