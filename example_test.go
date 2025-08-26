// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers_test

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/maruel/roundtrippers"
)

func Example_gET() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Check Accept-Encoding first!
		w.Header().Set("Content-Encoding", "zstd")
		c, err := zstd.NewWriter(w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = c.Write([]byte("Awesome"))
		if err = c.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	// Make all HTTP request in the current program:
	// - Retry on 429 and 5xx.
	// - Add a X-Request-ID for tracking both client and server side.
	// - Accept compressed responses with zstandard and brotli, in addition to gzip.
	// - Add logging.
	// - Add Authorization Bearer header.

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	const apiKey = "secret-key-that-will-not-appear-in-logs!"

	// Retry HTTP 429, 5xx.
	http.DefaultClient.Transport = &roundtrippers.Retry{
		// Add a unique X-Request-ID HTTP header to every requests.
		Transport: &roundtrippers.RequestID{
			// Accept brotli and zstd response in addition to gzip.
			Transport: &roundtrippers.AcceptCompressed{
				// Log requests via slog.
				Transport: &roundtrippers.Log{
					Logger: logger,
					// Authenticate.
					Transport: &roundtrippers.Header{
						Header:    http.Header{"Authorization": []string{"Bearer " + apiKey}},
						Transport: http.DefaultTransport,
					},
				},
			},
		},
	}

	// Now any request will be logged, authenticated and compressed.
	resp, err := http.Get(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Printf("GET: %s\n", string(b))
}

func Example_pOST() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ce := r.Header.Get("Content-Encoding"); ce != "gzip" {
			http.Error(w, "sorry, I only read gzip", http.StatusBadRequest)
			return
		}
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "error: "+err.Error(), http.StatusBadRequest)
			return
		}
		b, err := io.ReadAll(gz)
		if err != nil {
			http.Error(w, "error: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err = gz.Close(); err != nil {
			http.Error(w, "error: "+err.Error(), http.StatusBadRequest)
			return
		}
		if s := string(b); s != "hello" {
			http.Error(w, fmt.Sprintf("want \"hello\", got %q", s), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("world"))
	}))
	defer ts.Close()

	// Make all HTTP request in the current program:
	// - Retry on 429 and 5xx.
	// - Add a X-Request-ID for tracking both client and server side.
	// - Compress POST body with gzip.
	// - Accept compressed responses with zstandard and brotli, in addition to gzip.
	// - Add logging.
	// - Add Authorization Bearer header.

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	const apiKey = "secret-key-that-will-not-appear-in-logs!"

	// Retry HTTP 429, 5xx.
	http.DefaultClient.Transport = &roundtrippers.Retry{
		// Add a unique X-Request-ID HTTP header to every requests.
		Transport: &roundtrippers.RequestID{
			// Compress POST body with gzip
			Transport: &roundtrippers.PostCompressed{
				Encoding: "gzip",
				// Accept brotli and zstd response in addition to gzip.
				Transport: &roundtrippers.AcceptCompressed{
					// Log requests via slog.
					Transport: &roundtrippers.Log{
						Logger: logger,
						// Authenticate.
						Transport: &roundtrippers.Header{
							Header:    http.Header{"Authorization": []string{"Bearer " + apiKey}},
							Transport: http.DefaultTransport,
						},
					},
				},
			},
		},
	}

	// Now any request will be logged, authenticated and compressed, including POST request.
	resp, err := http.Post(ts.URL, "text/plain", strings.NewReader("hello"))
	if err != nil {
		log.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Printf("POST: %s\n", string(b))
}

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

func ExampleCapture_gET() {
	// Example on how to hook into the HTTP client roundtripper to capture each HTTP
	// response.
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
	fmt.Printf("Actual Response:   %q\n", string(b))
	record := <-ch
	fmt.Printf("Recorded Response: %q\n", record.Response.Body)

	// Output:
	// Actual Response:   "Working"
	// Recorded Response: {"Working"}
}

func ExampleCapture_pOST() {
	// Example on how to hook into the HTTP client roundtripper to capture each HTTP
	// request, including the POST body.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		_, _ = w.Write([]byte("Working"))
	}))
	defer ts.Close()

	ch := make(chan roundtrippers.Record, 1)
	t := &roundtrippers.Capture{Transport: http.DefaultTransport, C: ch}
	c := &http.Client{Transport: t}
	resp, err := c.Post(ts.URL, "text/plain", strings.NewReader("What are you doing?"))
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
	fmt.Printf("Actual Response:   %q\n", string(b))
	record := <-ch
	reqBodyReader, err := record.Request.GetBody()
	if err != nil {
		log.Fatal(err)
	}
	reqBody, err := io.ReadAll(reqBodyReader)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Recorded Request:  %q\n", reqBody)
	fmt.Printf("Recorded Response: %q\n", record.Response.Body)

	// Output:
	// Actual Response:   "Working"
	// Recorded Request:  "What are you doing?"
	// Recorded Response: {"Working"}
}

func ExampleHeader() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var names []string
		for k := range r.Header {
			if strings.HasPrefix(k, "Test-") {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		for _, k := range names {
			_, _ = fmt.Fprintf(w, "%s=%s\n", k, strings.Join(r.Header[k], ","))
		}
	}))
	defer ts.Close()
	h := http.Header{
		// A key with no value removes any pre-existing header with this key.
		"Test-Remove": nil,
		// A key with a single value will forcibly replace any preexisting value.
		"Test-Reset": []string{"value"},
		// A key with multiple values will append the values.
		"Test-Add": []string{"v1", "v2"},
	}
	c := http.Client{Transport: &roundtrippers.Header{Transport: http.DefaultTransport, Header: h}}
	resp, err := c.Get(ts.URL)
	resp.Header.Add("Test-Remove", "will be removed")
	resp.Header.Add("Test-Reset", "will be reset")
	resp.Header.Add("Test-Add", "will be kept")
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
	fmt.Printf("Response: %q\n", string(b))
	// Output:
	// Response: "Test-Add=v1,v2\nTest-Reset=value\n"
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

	t := &roundtrippers.RequestID{Transport: &roundtrippers.Log{Transport: http.DefaultTransport, Logger: logger}}
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

func ExamplePostCompressed_gzip() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ce := r.Header.Get("Content-Encoding"); ce != "gzip" {
			http.Error(w, "sorry, I only read gzip", http.StatusBadRequest)
			return
		}
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "error: "+err.Error(), http.StatusBadRequest)
			return
		}
		b, err := io.ReadAll(gz)
		if err != nil {
			http.Error(w, "error: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err = gz.Close(); err != nil {
			http.Error(w, "error: "+err.Error(), http.StatusBadRequest)
			return
		}
		if s := string(b); s != "hello" {
			http.Error(w, fmt.Sprintf("want \"hello\", got %q", s), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("world"))
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.PostCompressed{Transport: http.DefaultTransport, Encoding: "gzip"}}
	resp, err := c.Post(ts.URL, "text/plain", strings.NewReader("hello"))
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
	if s := string(b); s != "world" {
		log.Fatalf("want \"world\", got %q", s)
	}
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

func ExampleRetry() {
	count := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count++; count < 3 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
		} else {
			_, _ = w.Write([]byte("good"))
		}
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.Retry{
		Transport: http.DefaultTransport,
		// Optionally set a custom policy instead of roundtrippers.DefaultRetryPolicy.
		Policy: &roundtrippers.ExponentialBackoff{
			MaxTryCount: 10,
			MaxDuration: 60 * time.Second,
			Exp:         1.5,
		},
		// Disable sleeping for unit tests with this trick:
		TimeAfter: func(time.Duration) <-chan time.Time {
			c := make(chan time.Time, 1)
			c <- time.Now()
			return c
		},
	}}
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

func ExampleRetryPolicy() {
	c := http.Client{Transport: &roundtrippers.Retry{
		Transport: http.DefaultTransport,
		Policy: &PolicyCodes{
			RetryPolicy: &roundtrippers.DefaultRetryPolicy,
			Codes:       []int{http.StatusPaymentRequired},
		},
	}}
	// Call a web site that returns 402.
	_, _ = c.Get("http://example.com")
}

// PolicyCodes is a RetryPolicy that will retry on additional status codes.
type PolicyCodes struct {
	roundtrippers.RetryPolicy
	Codes []int
}

func (r *PolicyCodes) ShouldRetry(ctx context.Context, start time.Time, try int, err error, resp *http.Response) bool {
	if resp != nil && slices.Contains(r.Codes, resp.StatusCode) {
		return true
	}
	return r.RetryPolicy.ShouldRetry(ctx, start, try, err, resp)
}
