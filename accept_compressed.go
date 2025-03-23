// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// AcceptCompressed empowers the client to accept zstd, br and gzip compressed responses.
type AcceptCompressed struct {
	Transport http.RoundTripper

	_ struct{}
}

// RoundTrip implements http.RoundTripper.
func (a *AcceptCompressed) RoundTrip(req *http.Request) (*http.Response, error) {
	// The standard library includes gzip. Disable transparent compression and
	// add br and zstd. Tell the server we prefer zstd.
	req = req.Clone(req.Context())
	req.Header.Set("Accept-Encoding", "zstd, br, gzip")
	resp, err := a.Transport.RoundTrip(req)
	if resp != nil {
		// TODO: Handle "Content-Length" the same way stdlib does.
		switch ce := resp.Header.Get("Content-Encoding"); ce {
		case "br":
			resp.Body = &body{r: brotli.NewReader(resp.Body), c: []io.Closer{resp.Body}}
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
			resp.Uncompressed = true
		case "gzip":
			gz, err2 := gzip.NewReader(resp.Body)
			if err2 != nil {
				_ = resp.Body.Close()
				return nil, errors.Join(err2, err)
			}
			resp.Body = &body{r: gz, c: []io.Closer{resp.Body, gz}}
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
			resp.Uncompressed = true
		case "zstd":
			zs, err2 := zstd.NewReader(resp.Body)
			if err2 != nil {
				_ = resp.Body.Close()
				return nil, errors.Join(err2, err)
			}
			resp.Body = &body{r: zs, c: []io.Closer{resp.Body, &adapter{zs}}}
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
			resp.Uncompressed = true
		case "", "identity":
		default:
			_ = resp.Body.Close()
			return nil, fmt.Errorf("unsupported Content-Encoding %q", ce)
		}
	}
	return resp, err
}

func (a *AcceptCompressed) Unwrap() http.RoundTripper {
	return a.Transport
}

//

type adapter struct {
	zs *zstd.Decoder
}

func (a *adapter) Close() error {
	// zstd.Decoder doesn't implement io.Closer. :/
	a.zs.Close()
	return nil
}

type body struct {
	r io.Reader
	c []io.Closer
}

func (b *body) Read(p []byte) (n int, err error) {
	return b.r.Read(p)
}

func (b *body) Close() error {
	var errs []error
	// Close in reverse order.
	for i := len(b.c) - 1; i >= 0; i-- {
		if err := b.c[i].Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
