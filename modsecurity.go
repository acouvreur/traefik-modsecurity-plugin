// Package traefik_modsecurity_plugin a modsecurity plugin.
package traefik_modsecurity_plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// Config the plugin configuration.
type Config struct {
	TimeoutMillis  int64  `json:"timeoutMillis"`
	ModSecurityUrl string `json:"modSecurityUrl,omitempty"`
	MaxBodySize    int64  `json:"maxBodySize"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		TimeoutMillis: 2000,
		// Safe default: if the max body size was not specified, use 10MB
		// Note that this will break any file upload with files > 10MB. Hopefully
		// the user will configure this parameter during the installation.
		MaxBodySize: 10 * 1024 * 1024,
	}
}

// Modsecurity a Modsecurity plugin.
type Modsecurity struct {
	next           http.Handler
	modSecurityUrl string
	maxBodySize    int64
	name           string
	httpClient     *http.Client
	logger         *log.Logger
}

// New created a new Modsecurity plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.ModSecurityUrl) == 0 {
		return nil, fmt.Errorf("modSecurityUrl cannot be empty")
	}

	// Use a custom client with predefined timeout ot 2 seconds
	var timeout time.Duration
	if config.TimeoutMillis == 0 {
		timeout = 2 * time.Second
	} else {
		timeout = time.Duration(config.TimeoutMillis) * time.Millisecond
	}

	return &Modsecurity{
		modSecurityUrl: config.ModSecurityUrl,
		maxBodySize:    config.MaxBodySize,
		next:           next,
		name:           name,
		httpClient:     &http.Client{Timeout: timeout},
		logger:         log.New(os.Stdout, "", log.LstdFlags),
	}, nil
}

func (a *Modsecurity) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	// Websocket not supported
	if isWebsocket(req) {
		a.next.ServeHTTP(rw, req)
		return
	}

	// we need to buffer the body if we want to read it here and send it
	// in the request.
	body, err := ioutil.ReadAll(http.MaxBytesReader(rw, req.Body, a.maxBodySize))
	if err != nil {
		if err.Error() == "http: request body too large" {
			a.logger.Printf("body max limit reached: %s", err.Error())
			http.Error(rw, "", http.StatusRequestEntityTooLarge)
		} else {
			a.logger.Printf("fail to read incoming request: %s", err.Error())
			http.Error(rw, "", http.StatusBadGateway)
		}
		return
	}

	// you can reassign the body if you need to parse it as multipart
	req.Body = ioutil.NopCloser(bytes.NewReader(body))

	// create a new url from the raw RequestURI sent by the client
	url := fmt.Sprintf("%s%s", a.modSecurityUrl, req.RequestURI)

	proxyReq, err := http.NewRequest(req.Method, url, bytes.NewReader(body))

	if err != nil {
		a.logger.Printf("fail to prepare forwarded request: %s", err.Error())
		http.Error(rw, "", http.StatusBadGateway)
		return
	}

	// We may want to filter some headers, otherwise we could just use a shallow copy
	// proxyReq.Header = req.Header
	proxyReq.Header = make(http.Header)
	for h, val := range req.Header {
		proxyReq.Header[h] = val
	}

	resp, err := a.httpClient.Do(proxyReq)
	if err != nil {
		a.logger.Printf("fail to send HTTP request to modsec: %s", err.Error())
		http.Error(rw, "", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		forwardResponse(resp, rw)
		return
	}

	a.next.ServeHTTP(rw, req)
}

func isWebsocket(req *http.Request) bool {
	for _, header := range req.Header["Upgrade"] {
		if header == "websocket" {
			return true
		}
	}
	return false
}

func forwardResponse(resp *http.Response, rw http.ResponseWriter) {
	// copy headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			rw.Header().Set(k, v)
		}
	}
	// copy status
	rw.WriteHeader(resp.StatusCode)
	// copy body
	io.Copy(rw, resp.Body)
}
