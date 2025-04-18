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

// PostCompressed empowers the client to POST zstd, br and gzip compressed requests.
type PostCompressed struct {
	Transport http.RoundTripper
	// Encoding determines HTTP POST compression. It must be empty or one of: "zstd", "br" or "zstd".
	//
	// Warning ⚠: compressing POST content is not supported on most servers.
	Encoding string
	// Level is the compression level.
	// - "br" uses values between 1 and 11. If unset, defaults to 3.
	// - "gzip" uses values between 1 and 9. If unset, defaults to 3.
	// - "zstd"  uses values between 1 and 4. If unset, defaults to 2.
	Level int

	_ struct{}
}

// RoundTrip implements http.RoundTripper.
func (p *PostCompressed) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil || req.Header.Get("Content-Encoding") != "" {
		return p.Transport.RoundTrip(req)
	}
	req2 := req.Clone(req.Context())
	var err error
	if req2.Body, err = p.getBody(req.Body); err != nil {
		return nil, err
	}
	if req.GetBody != nil {
		req2.GetBody = func() (io.ReadCloser, error) {
			b2, err2 := req.GetBody()
			if err2 != nil {
				return b2, err2
			}
			return p.getBody(b2)
		}
	} else {
		// See https://github.com/golang/go/issues/73439
		req2.GetBody = nil
	}
	req2.ContentLength = -1
	req2.Header.Del("Content-Length")
	req2.Header.Set("Content-Encoding", p.Encoding)
	return p.Transport.RoundTrip(req2)
}

func (p *PostCompressed) Unwrap() http.RoundTripper {
	return p.Transport
}

func (p *PostCompressed) getBody(oldBody io.ReadCloser) (io.ReadCloser, error) {
	r, w := io.Pipe()
	switch p.Encoding {
	case "gzip":
		go func() {
			l := p.Level
			if l == 0 {
				// Use a fast compression level.
				l = 3
			}
			gz, err := gzip.NewWriterLevel(w, l)
			if err != nil {
				_ = oldBody.Close()
				_ = w.CloseWithError(err)
				return
			}
			_, err = io.Copy(gz, oldBody)
			if err2 := oldBody.Close(); err == nil {
				err = err2
			}
			if err2 := gz.Close(); err == nil {
				err = err2
			}
			if err != nil {
				_ = w.CloseWithError(err)
				return
			}
			_ = w.Close()
		}()
		return r, nil
	case "br":
		go func() {
			l := p.Level
			if l == 0 {
				// Use a fast compression level.
				l = 3
			}
			br := brotli.NewWriterLevel(w, l)
			_, err := io.Copy(br, oldBody)
			if err2 := oldBody.Close(); err == nil {
				err = err2
			}
			if err2 := br.Close(); err == nil {
				err = err2
			}
			if err != nil {
				_ = w.CloseWithError(err)
				return
			}
			_ = w.Close()
		}()
		return r, nil
	case "zstd":
		go func() {
			l := zstd.EncoderLevel(p.Level)
			if l == 0 {
				// Use a fast compression level.
				l = zstd.SpeedFastest
			}
			zs, err := zstd.NewWriter(w, zstd.WithEncoderLevel(l))
			if err != nil {
				_ = oldBody.Close()
				_ = w.CloseWithError(err)
				return
			}
			_, err = io.Copy(zs, oldBody)
			if err2 := oldBody.Close(); err == nil {
				err = err2
			}
			if err2 := zs.Close(); err == nil {
				err = err2
			}
			if err != nil {
				_ = w.CloseWithError(err)
				return
			}
			_ = w.Close()
		}()
		return r, nil
	case "":
		return nil, errors.New("do not use PostCompressed without Encoding")
	}
	return nil, fmt.Errorf("invalid Encoding value: %q", p.Encoding)
}
