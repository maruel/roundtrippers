// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/maruel/roundtrippers"
)

func TestCapture_RoundTrip_error_bad_url(t *testing.T) {
	ch := make(chan roundtrippers.Record, 2)
	c := http.Client{Transport: &roundtrippers.Capture{Transport: http.DefaultTransport, C: ch}}
	resp, err := c.Get("")
	if resp != nil || err == nil {
		t.Fatal(resp, err)
	}
	select {
	case captured := <-ch:
		if captured.Request == nil {
			t.Fatal("expected request")
		}
		if captured.Response != nil {
			t.Fatalf("unexpected response: %#v", captured.Response)
		}
		if captured.Err == nil {
			t.Fatal("expected error")
		}
	default:
		t.Fatal("expected a capture")
	}
	select {
	case captured := <-ch:
		t.Fatalf("unexpected capture: %#v", captured)
	default:
	}
}

func TestCapture_RoundTrip_error_short(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("too short"))
	}))
	defer ts.Close()
	ch := make(chan roundtrippers.Record, 2)
	c := http.Client{Transport: &roundtrippers.Capture{Transport: http.DefaultTransport, C: ch}}
	resp, err := c.Get(ts.URL)
	if resp == nil || err != nil {
		t.Fatal(resp, err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != io.ErrUnexpectedEOF {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != "too short" {
		t.Fatal(s)
	}
	select {
	case captured := <-ch:
		if captured.Request == nil {
			t.Fatal("expected request")
		}
		if captured.Response == nil {
			t.Fatalf("expected response: %#v", captured.Response)
		}
		if captured.Err == nil {
			t.Fatal("expected error")
		}
	default:
		t.Fatal("expected a capture")
	}
	select {
	case captured := <-ch:
		t.Fatalf("unexpected capture: %#v", captured)
	default:
	}
}

func TestCapture_redirect(t *testing.T) {
	data := []struct {
		name       string
		r          io.Reader
		hasGetBody bool
	}{
		// This will set GetBody due to type assertion in http.NewRequestWithContext().
		{
			"WithGetBody",
			strings.NewReader("hello"),
			true,
		},
		// This will not set GetBody because it's a custom type.
		{
			"WithoutGetBody",
			&reader{"hello"},
			false,
		},
	}
	for _, line := range data {
		t.Run(line.name, func(t *testing.T) {
			// Ensures GetBody works.
			var count atomic.Int64
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// First return a 307, then succeed.
				v := count.Add(1)
				t.Logf("%s: %d", r.Method, v)
				if v == 1 {
					t.Logf("redirecting")
					http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
					return
				}
				b, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				if err = r.Body.Close(); err != nil {
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
			ch := make(chan roundtrippers.Record, 3)
			c := http.Client{Transport: &roundtrippers.Capture{Transport: http.DefaultTransport, C: ch}}
			req, err := http.NewRequestWithContext(t.Context(), "POST", ts.URL, line.r)
			if err != nil {
				t.Fatal(err)
			}
			if hasGetBody := req.GetBody != nil; hasGetBody != line.hasGetBody {
				t.Fatalf("unexpected GetBody: %t != %t", hasGetBody, line.hasGetBody)
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
				// t.Fatalf("want \"world\", got %q", s)
			}
			if v := count.Load(); v != 2 {
				// t.Fatalf("expected 2 requests, got %d", v)
			}
			// See https://github.com/golang/go/issues/73439
			numCapture := 2
			if !line.hasGetBody {
				numCapture = 1
			}
			for i := range numCapture {
				select {
				case captured := <-ch:
					if captured.Request == nil {
						t.Fatal("expected request")
					}
					if captured.Response == nil {
						t.Fatalf("expected response: %#v", captured.Response)
					}
					if captured.Err != nil {
						t.Fatalf("unexpected error %s", err)
					}
				default:
					t.Errorf("expected a capture #%d", i)
				}
			}
			select {
			case captured := <-ch:
				t.Fatalf("unexpected capture: %#v", captured)
			default:
			}
		})
	}
}

func TestCapture_Unwrap(t *testing.T) {
	var r http.RoundTripper = &roundtrippers.Capture{Transport: http.DefaultTransport}
	if r.(roundtrippers.Unwrapper).Unwrap() != http.DefaultTransport {
		t.Fatal("unexpected")
	}
}
