// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/maruel/roundtrippers"
)

func TestPostCompressed_error_compress_bad(t *testing.T) {
	c := http.Client{Transport: &roundtrippers.PostCompressed{Transport: http.DefaultTransport, Encoding: "bad"}}
	if _, err := c.Post("http://127.0.0.1:0", "text/plain", strings.NewReader("hello")); err == nil {
		t.Fatal("expected error")
	}
}

func TestPostCompressed_error_compress_missing(t *testing.T) {
	c := http.Client{Transport: &roundtrippers.PostCompressed{Transport: http.DefaultTransport, Encoding: ""}}
	if _, err := c.Post("http://127.0.0.1:0", "text/plain", strings.NewReader("hello")); err == nil {
		t.Fatal("expected error")
	}
}

func TestPostCompressed_get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("world"))
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.PostCompressed{Transport: http.DefaultTransport, Encoding: "gzip"}}
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != "world" {
		t.Fatalf("want \"world\", got %q", s)
	}
}

func TestPostCompressed_gzip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ce := r.Header.Get("Content-Encoding"); ce != "gzip" {
			t.Error(ce)
			return
		}
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		b, err := io.ReadAll(gz)
		if err != nil {
			t.Error(err, b)
			return
		}
		if err = gz.Close(); err != nil {
			t.Error(err)
		}
		if s := string(b); s != "hello" {
			t.Errorf("want \"hello\", got %q", s)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("world"))
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.PostCompressed{Transport: http.DefaultTransport, Encoding: "gzip"}}
	resp, err := c.Post(ts.URL, "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != "world" {
		t.Fatalf("want \"world\", got %q", s)
	}
}

func TestPostCompressed_br(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ce := r.Header.Get("Content-Encoding"); ce != "br" {
			t.Error(ce)
			return
		}
		br := brotli.NewReader(r.Body)
		b, err := io.ReadAll(br)
		if err != nil {
			t.Error(err, b)
			return
		}
		if s := string(b); s != "hello" {
			t.Errorf("want \"hello\", got %q", s)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("world"))
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.PostCompressed{Transport: http.DefaultTransport, Encoding: "br"}}
	resp, err := c.Post(ts.URL, "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != "world" {
		t.Fatalf("want \"world\", got %q", s)
	}
}

func TestPostCompressed_zstd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ce := r.Header.Get("Content-Encoding"); ce != "zstd" {
			t.Error(ce)
			return
		}
		zs, err := zstd.NewReader(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		b, err := io.ReadAll(zs)
		if err != nil {
			t.Error(err, b)
			return
		}
		zs.Close()
		if s := string(b); s != "hello" {
			t.Errorf("want \"hello\", got %q", s)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("world"))
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.PostCompressed{Transport: http.DefaultTransport, Encoding: "zstd"}}
	resp, err := c.Post(ts.URL, "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != "world" {
		t.Fatalf("want \"world\", got %q", s)
	}
}

func TestPostCompressed_redirect(t *testing.T) {
	// Ensures GetBody works.
	var count atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First return a 307, then succeed.
		v := count.Add(1)
		t.Logf("%s: %d", r.Method, v)
		if v == 1 {
			http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
			return
		}
		if ce := r.Header.Get("Content-Encoding"); ce != "zstd" {
			t.Error(ce)
			return
		}
		zs, err := zstd.NewReader(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		b, err := io.ReadAll(zs)
		if err != nil {
			t.Error(err, b)
			return
		}
		zs.Close()
		if s := string(b); s != "hello" {
			t.Errorf("want \"hello\", got %q", s)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("world"))
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.PostCompressed{Transport: http.DefaultTransport, Encoding: "zstd"}}
	req, err := http.NewRequestWithContext(t.Context(), "POST", ts.URL, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if req.GetBody == nil {
		t.Fatal("expected stdlib to set GetBody automatically")
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != "world" {
		t.Fatalf("want \"world\", got %q", s)
	}
	if v := count.Load(); v != 2 {
		t.Fatalf("expected 2 requests, got %d", v)
	}
}

func TestPostCompressed_Unwrap(t *testing.T) {
	var r http.RoundTripper = &roundtrippers.PostCompressed{Transport: http.DefaultTransport}
	if r.(roundtrippers.Unwrap).Unwrap() != http.DefaultTransport {
		t.Fatal("unexpected")
	}
}
