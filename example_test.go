// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers_test

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/maruel/roundtrippers"
)

func acceptCompressed(r *http.Request, want string) bool {
	for encoding := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(encoding) == want {
			return true
		}
	}
	return false
}

func ExampleAcceptCompressed_br() {
	// Example on how to hook into the HTTP client roundtripper to enable zstd and brotli.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !acceptCompressed(r, "br") {
			http.Error(w, "sorry, I only talk br", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Encoding", "br")
		c := brotli.NewWriter(w)
		_, _ = c.Write([]byte("excellent"))
		if err := c.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	t := &roundtrippers.AcceptCompressed{Transport: http.DefaultTransport}
	c := http.Client{Transport: t}
	resp, err := c.Get(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %q\n", string(b))
	// Output:
	// Response: "excellent"
}

func ExampleAcceptCompressed_gzip() {
	// Example on how to hook into the HTTP client roundtripper to enable zstd and brotli.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !acceptCompressed(r, "gzip") {
			http.Error(w, "sorry, I only talk gzip", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		c := gzip.NewWriter(w)
		_, _ = c.Write([]byte("excellent"))
		if err := c.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	t := &roundtrippers.AcceptCompressed{Transport: http.DefaultTransport}
	c := http.Client{Transport: t}
	resp, err := c.Get(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %q\n", string(b))
	// Output:
	// Response: "excellent"
}

func ExampleAcceptCompressed_zstd() {
	// Example on how to hook into the HTTP client roundtripper to enable zstd and brotli.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !acceptCompressed(r, "zstd") {
			http.Error(w, "sorry, I only talk zstd", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Encoding", "zstd")
		c, err := zstd.NewWriter(w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = c.Write([]byte("excellent"))
		if err = c.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	t := &roundtrippers.AcceptCompressed{Transport: http.DefaultTransport}
	c := http.Client{Transport: t}
	resp, err := c.Get(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %q\n", string(b))
	// Output:
	// Response: "excellent"
}

func ExampleLog() {
	// Example on how to hook into the HTTP client roundtripper to log each HTTP
	// request.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Working"))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout,
		&slog.HandlerOptions{
			Level: slog.LevelDebug,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				// For testing reproducibility, remove the timestamp, url, request id and duration.
				if a.Key == "time" || a.Key == "url" || a.Key == "id" || a.Key == "dur" {
					return slog.Attr{}
				}
				return a
			},
		}))

	t := &roundtrippers.RequestID{Transport: &roundtrippers.Log{Transport: http.DefaultTransport, L: logger}}
	c := http.Client{Transport: t}

	resp, err := c.Get(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %q\n", string(b))
	// Output:
	// level=INFO msg=http method=GET Content-Encoding=""
	// level=INFO msg=http status=200 Content-Encoding="" Content-Length=7 Content-Type="text/plain; charset=utf-8"
	// level=INFO msg=http size=7 err=<nil>
	// Response: "Working"
}

func ExampleCapture() {
	// Example on how to hook into the HTTP client roundtripper to capture each HTTP
	// request.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Working"))
	}))
	defer ts.Close()

	ch := make(chan roundtrippers.Record, 1)
	t := &roundtrippers.Capture{Transport: http.DefaultTransport, C: ch}
	c := &http.Client{Transport: t}
	resp, err := c.Get(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		log.Fatal(err)
	}

	// Print the captured request and response.
	fmt.Printf("Response: %q\n", string(b))
	record := <-ch
	fmt.Printf("Recorded: %q\n", record.Response.Body)

	// Output:
	// Response: "Working"
	// Recorded: {"Working"}
}

func ExampleRequestID() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			_, _ = w.Write([]byte("bad"))
		} else {
			_, _ = w.Write([]byte("good"))
		}
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.RequestID{Transport: http.DefaultTransport}}
	resp, err := c.Get(ts.URL)
	if resp == nil || err != nil {
		log.Fatal(resp, err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %q\n", string(b))
	// Output:
	// Response: "good"
}
