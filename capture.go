// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"bytes"
	"io"
	"net/http"
)

// Record is a captured HTTP request and response by the Capture http.RoundTripper.
type Record struct {
	// Request is guaranteed to have GetBody set is Body was set. Use this to read the POST's body.
	Request  *http.Request
	Response *http.Response
	// Err is the error returned by the http.RoundTripper.Do(), if any.
	Err error

	_ struct{}
}

// Capture is a http.RoundTripper that records each request.
type Capture struct {
	Transport http.RoundTripper
	C         chan<- Record

	_ struct{}
}

// RoundTrip implements http.RoundTripper.
func (c *Capture) RoundTrip(req *http.Request) (*http.Response, error) {
	// Ensures GetBody is set, so the user can read this.
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		if req, err = cloneRequestWithBody(req); err != nil {
			return nil, err
		}
	}
	resp, err := c.Transport.RoundTrip(req)
	if resp != nil {
		// Make a copy of the response.
		resp2 := &http.Response{}
		*resp2 = *resp
		resp.Body = &captureBody{
			body:    resp.Body,
			req:     req,
			resp:    resp2,
			c:       c.C,
			content: &bytes.Buffer{},
		}
	} else {
		c.C <- Record{Request: req, Err: err}
	}
	return resp, err
}

func (c *Capture) Unwrap() http.RoundTripper {
	return c.Transport
}

//

type captureBody struct {
	body    io.ReadCloser
	req     *http.Request
	resp    *http.Response
	c       chan<- Record
	content *bytes.Buffer
	err     error
}

func (c *captureBody) Read(p []byte) (int, error) {
	n, err := c.body.Read(p)
	_, _ = c.content.Write(p[:n])
	if err != nil && err != io.EOF && c.err == nil {
		c.err = err
	}
	return n, err
}

func (c *captureBody) Close() error {
	err := c.body.Close()
	c.resp.Body = io.NopCloser(c.content)
	// The Request object in the Response may be different from what we saved.
	c.c <- Record{Request: c.req, Response: c.resp, Err: c.err}
	return err
}
