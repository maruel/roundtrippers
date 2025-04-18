// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_error_compress_bad(t *testing.T) {
	c := http.Client{Transport: &Retry{Transport: http.DefaultTransport}}
	if _, err := c.Post("http://127.0.0.1:0", "text/plain", strings.NewReader("hello")); err == nil {
		t.Fatal("expected error")
	}
}

func TestRetry_get(t *testing.T) {
	var count atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := count.Add(1)
		t.Logf("%s: %d", r.Method, v)
		if v == 1 {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte("hi"))
	}))
	defer ts.Close()
	c := http.Client{Transport: &Retry{Transport: http.DefaultTransport}}
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
	if s := string(b); s != "hi" {
		t.Fatalf("want \"hi\", got %q", s)
	}
	if v := count.Load(); v != 2 {
		t.Fatalf("expected 2 tries, got %d", v)
	}
}

func TestRetry_redirect_infinite(t *testing.T) {
	var count atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		http.Redirect(w, r, "/again", http.StatusTemporaryRedirect)
	}))
	defer ts.Close()
	c := http.Client{Transport: &Retry{Transport: http.DefaultTransport}}
	resp, err := c.Get(ts.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if v := count.Load(); v != 10 {
		t.Fatalf("expected 10 tries, got %d", v)
	}
}

func TestRetry_invalid_scheme(t *testing.T) {
	var count atomic.Int64
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		// It doesn't trigger the code I want it to trigger.
		http.Redirect(w, r, "foo://"+ts.Listener.Addr().String(), http.StatusTemporaryRedirect)
	}))
	defer ts.Close()
	c := http.Client{Transport: &Retry{Transport: http.DefaultTransport}}
	resp, err := c.Get(ts.URL)
	if resp != nil || err == nil {
		t.Fatal(err)
	}
	if v := count.Load(); v != 1 {
		t.Fatalf("expected 1 tries, got %d", v)
	}
}

func TestRetry_invalid_cert(t *testing.T) {
	var count atomic.Int64
	ts2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		_, _ = w.Write([]byte("hi"))
	}))
	defer ts2.Close()
	ts2.Config.ErrorLog = log.New(io.Discard, "", 0)
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		// It doesn't trigger the code I want it to trigger.
		http.Redirect(w, r, ts2.URL, http.StatusTemporaryRedirect)
	}))
	defer ts1.Close()
	c := http.Client{Transport: &Retry{Transport: http.DefaultTransport}}
	resp, err := c.Get(ts1.URL)
	if resp != nil || err == nil {
		t.Fatal(err)
	}
	if v := count.Load(); v != 1 {
		t.Fatalf("expected 1 tries, got %d", v)
	}
}

func TestRetry_invalid_protocol(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			t.Fatalf("failed to listen on a port: %v", err)
		}
	}
	defer l.Close()
	go func() {
		c, err2 := l.Accept()
		if err2 != nil {
			t.Error(err2)
			return
		}
		// It doesn't trigger the code I want it to trigger.
		c.Write([]byte("HTTP/0.0 99\r\n"))
		c.Close()
	}()
	c := http.Client{Transport: &Retry{Transport: http.DefaultTransport}}
	resp, err := c.Get("http://" + l.Addr().String())
	if resp != nil || err == nil {
		t.Fatal(err)
	}
}

func TestRetry_post(t *testing.T) {
	data := []struct {
		r    io.Reader
		code int
		hdr  http.Header
	}{
		// This will set GetBody due to type assertion in http.NewRequestWithContext().
		{strings.NewReader("hello"), 503, nil},
		{strings.NewReader("hello"), 429, http.Header{"Retry-After": []string{"1"}}},
		{
			strings.NewReader("hello"),
			429,
			http.Header{"Retry-After": []string{time.Now().Add(time.Second).Format(time.RFC1123)}},
		},
		// This will not set GetBody because it's a custom type.
		{&reader{"hello"}, 503, nil},
	}
	for i, line := range data {
		t.Run(fmt.Sprintf("%d-%d", i, line.code), func(t *testing.T) {
			var count atomic.Int64
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				_ = r.Body.Close()
				v := count.Add(1)
				t.Logf("%s: %d", r.Method, v)
				if v == 1 {
					for k, values := range line.hdr {
						for _, v := range values {
							t.Logf("%s=%s", k, v)
							w.Header().Set(k, v)
						}
					}
					w.WriteHeader(line.code)
					return
				}
				_, _ = w.Write([]byte("hi"))
			}))
			defer ts.Close()
			c := http.Client{Transport: &Retry{Transport: http.DefaultTransport}}
			resp, err := c.Post(ts.URL, "text/plain", line.r)
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
			if s := string(b); s != "hi" {
				t.Fatalf("want \"hi\", got %q", s)
			}
			if v := count.Load(); v != 2 {
				t.Fatalf("expected 2 tries, got %d", v)
			}
		})
	}
}

func TestRetry_Unwrap(t *testing.T) {
	var r http.RoundTripper = &Retry{Transport: http.DefaultTransport}
	if r.(Unwrapper).Unwrap() != http.DefaultTransport {
		t.Fatal("unexpected")
	}
}

//

type reader struct {
	s string
}

func (r *reader) Read(b []byte) (int, error) {
	i := copy(b, r.s)
	if r.s = r.s[i:]; len(r.s) == 0 {
		return i, io.EOF
	}
	return i, nil
}

func init() {
	timeAfter = func(time.Duration) <-chan time.Time {
		c := make(chan time.Time, 1)
		c <- time.Now()
		return c
	}
}
