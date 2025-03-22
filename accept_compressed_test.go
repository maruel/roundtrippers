// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/maruel/roundtrippers"
)

func TestAcceptCompressed_RoundTrip_error_bad_url(t *testing.T) {
	c := http.Client{Transport: &roundtrippers.AcceptCompressed{Transport: http.DefaultTransport}}
	resp, err := c.Get("")
	if resp != nil || err == nil {
		t.Fatal(resp, err)
	}
}

func TestAcceptCompressed_RoundTrip_error_short(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !acceptCompressed(r, "zstd") {
			http.Error(w, "sorry, I only talk zstd", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Encoding", "zstd")
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		c, err := zstd.NewWriter(w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = c.Write([]byte("too short"))
		if err = c.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.AcceptCompressed{Transport: http.DefaultTransport}}
	resp, err := c.Get(ts.URL)
	if resp == nil || err != nil {
		t.Fatal(resp, err)
	}
	b, err := io.ReadAll(resp.Body)
	// BUG: Should be io.ErrUnexpectedEOF.
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != "too short" {
		t.Fatal(s)
	}
}

func TestAcceptCompressed_identity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "identity")
		_, _ = w.Write([]byte("excellent"))
	}))
	defer ts.Close()

	c := http.Client{Transport: &roundtrippers.AcceptCompressed{Transport: http.DefaultTransport}}
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != "excellent" {
		t.Fatal(s)
	}
}

func TestAcceptCompressed_error_bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "new_quantum_compression")
		_, _ = w.Write([]byte("excellent"))
	}))
	defer ts.Close()

	c := http.Client{Transport: &roundtrippers.AcceptCompressed{Transport: http.DefaultTransport}}
	resp, err := c.Get(ts.URL)
	if resp != nil || err == nil {
		t.Fatal(resp, err)
	}
}
