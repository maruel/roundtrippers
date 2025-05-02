// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package roundtrippers is a collection of high quality http.RoundTripper to
// augment your http.Client.
package roundtrippers

import (
	"bytes"
	"io"
	"net/http"
)

// Unwrapper enables users to get the underlying transport when wrapped with a middleware.
type Unwrapper interface {
	Unwrap() http.RoundTripper
}

// cloneRequestWithBody clones the request and ensures the http.Request has a GetBody if Body is set.
func cloneRequestWithBody(req *http.Request) (*http.Request, error) {
	req2 := req.Clone(req.Context())
	// See https://github.com/golang/go/issues/73439
	req2.GetBody = req.GetBody
	if req2.Body != nil && req2.GetBody == nil {
		in, err := io.ReadAll(req2.Body)
		if err != nil {
			return nil, err
		}
		req2.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBuffer(in)), nil
		}
		req2.Body, _ = req2.GetBody()
	}
	return req2, nil
}
