package main

import (
	"fmt"
	"net/http"
)

type tlsOnlyTransport struct{}

func (tlsOnlyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || r.URL.User != nil {
		return nil, fmt.Errorf("HTTPS with a trusted certificate is required")
	}
	return http.DefaultTransport.RoundTrip(r)
}

func sameAgentOrigin(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 || len(via) == 0 || req.URL.Scheme != "https" || req.URL.Host != via[0].URL.Host {
		return fmt.Errorf("agent redirect outside trusted origin denied")
	}
	return nil
}
