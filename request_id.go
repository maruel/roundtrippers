// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

// RequestID is a http.RoundTripper that add X-Request-ID to each request. It
// can be useful to track requests simultaneously on the client and the server
// or for logging purposes.
type RequestID struct {
	Transport http.RoundTripper

	_ struct{}
}

func (r *RequestID) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Request-ID", genID())
	return r.Transport.RoundTrip(req)
}

func genID() string {
	var bytes [12]byte
	_, _ = rand.Read(bytes[:])
	return base64.RawURLEncoding.EncodeToString(bytes[:])
}
