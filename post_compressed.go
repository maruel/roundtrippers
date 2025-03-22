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

	_ struct{}
}

func (p *PostCompressed) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil || req.Header.Get("Content-Encoding") != "" {
		return p.Transport.RoundTrip(req)
	}
	req = req.Clone(req.Context())
	oldBody := req.Body
	r, w := io.Pipe()
	switch p.Encoding {
	case "gzip":
		go func() {
			// Use a fast compression level.
			gz, err := gzip.NewWriterLevel(w, 3)
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
	case "br":
		go func() {
			// Use a fast compression level.
			br := brotli.NewWriterLevel(w, 3)
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
	case "zstd":
		go func() {
			zs, err := zstd.NewWriter(w)
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
	case "":
		return nil, errors.New("do not use PostCompressed without Encoding")
	default:
		return nil, fmt.Errorf("invalid Encoding value: %q", p.Encoding)
	}
	req.Body = r
	req.GetBody = nil
	req.ContentLength = -1
	req.Header.Del("Content-Length")
	req.Header.Set("Content-Encoding", p.Encoding)
	return p.Transport.RoundTrip(req)
}
