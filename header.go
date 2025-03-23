// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"net/http"
)

// Header is a http.RoundTripper that adds a set of headers to each request./
//
// It is useful to set the Authorization bearer token to all requests
// simultaneously on the client.
type Header struct {
	Transport http.RoundTripper
	// Header is the headers to add or remove.
	// - A key with no value will remove the key from the HTTP request.
	// - A key with one value will reset the value for this key.
	// - A key with multiple values will append the values to the preexisting ones, if any.
	Header http.Header

	_ struct{}
}

// RoundTrip implements http.RoundTripper.
func (h *Header) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range h.Header {
		switch len(v) {
		case 0:
			req.Header.Del(k)
		case 1:
			req.Header.Set(k, v[0])
		default:
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
	}
	return h.Transport.RoundTrip(req)
}

func (h *Header) Unwrap() http.RoundTripper {
	return h.Transport
}
