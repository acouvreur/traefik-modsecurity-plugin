package traefik_modsecurity_plugin

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// HandleRequestBodyMaxSize - handle request body max size - TODO find a better way to do this
func (a *Modsecurity) HandleRequestBodyMaxSize(rw http.ResponseWriter, req *http.Request) error {
	bodyReader := http.MaxBytesReader(rw, req.Body, a.maxBodySize+1)
	n, err := io.Copy(ioutil.Discard, bodyReader)
	req.Body.Close()

	if err != nil {
		if err.Error() == "http: request body too large" {
			a.logger.Printf("body max limit reached: %s", err.Error())
			http.Error(rw, "", http.StatusRequestEntityTooLarge)
		} else {
			a.logger.Printf("fail to read incoming request: %s", err.Error())
			http.Error(rw, "", http.StatusBadGateway)
		}
		return err
	}

	if n > a.maxBodySize {
		a.logger.Printf("body max limit reached: content length %d is larger than the allowed limit %d", n, a.maxBodySize)
		http.Error(rw, "", http.StatusRequestEntityTooLarge)
		return fmt.Errorf("http: request body too large")
	}

	return nil
}

func (a *Modsecurity) HandleCacheAndForwardRequest(req *http.Request) (*http.Response, error) {
	var resp *http.Response

	//if cache is disabled, immediately forward to modsecurity
	if !a.cacheEnabled {
		return a.PrepareForwardedRequest(req)
	}

	//get from our memory cache if possible
	if a.cacheConditions.Check(req) {
		resp, err := a.GetCachedResponse(req, a.cacheKey)
		// a.logger.Printf("cache hit: %v", err == nil)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	//forward to modsecurity
	resp, err := a.PrepareForwardedRequest(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
