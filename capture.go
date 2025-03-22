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
	Request  *http.Request
	Response *http.Response
	Err      error

	_ struct{}
}

// Capture is a http.RoundTripper that records each request.
type Capture struct {
	Transport http.RoundTripper
	C         chan<- Record

	_ struct{}
}

func (c *Capture) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.Transport.RoundTrip(req)
	if resp != nil {
		// Make a copy of the response.
		resp2 := &http.Response{}
		*resp2 = *resp
		resp.Body = &captureBody{
			body:    resp.Body,
			resp:    resp2,
			c:       c.C,
			content: &bytes.Buffer{},
		}
	} else {
		c.C <- Record{Request: req, Err: err}
	}
	return resp, err
}

type captureBody struct {
	body    io.ReadCloser
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

func (l *captureBody) Close() error {
	err := l.body.Close()
	l.resp.Body = io.NopCloser(l.content)
	l.c <- Record{Request: l.resp.Request, Response: l.resp, Err: l.err}
	return err
}
