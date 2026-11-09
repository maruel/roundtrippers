// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Test HTTP request and response logs.

package roundtrippers_test

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maruel/roundtrippers"
)

func TestLog(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		t.Run("error", func(t *testing.T) {
			t.Run("bad_url", func(t *testing.T) {
				c := http.Client{Transport: &roundtrippers.RequestID{Transport: &roundtrippers.Log{Transport: http.DefaultTransport, Logger: slog.New(slog.DiscardHandler)}}}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := c.Do(req)
				if resp != nil {
					if err2 := resp.Body.Close(); err2 != nil {
						t.Error(err2)
					}
				}
				if resp != nil || err == nil {
					t.Fatal(resp, err)
				}
			})

			t.Run("missing_request_id", func(t *testing.T) {
				c := http.Client{Transport: &roundtrippers.Log{Transport: http.DefaultTransport, Logger: slog.New(slog.DiscardHandler)}}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := c.Do(req)
				if resp != nil {
					if err2 := resp.Body.Close(); err2 != nil {
						t.Error(err2)
					}
				}
				if resp != nil || err == nil {
					t.Fatal(resp, err)
				}
			})

			t.Run("short", func(t *testing.T) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Length", "1024")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("too short"))
				}))
				defer ts.Close()
				c := http.Client{Transport: &roundtrippers.RequestID{Transport: &roundtrippers.Log{Transport: http.DefaultTransport, Logger: slog.New(slog.DiscardHandler)}}}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, http.NoBody)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := c.Do(req)
				if resp == nil || err != nil {
					t.Fatal(resp, err)
				}
				b, err := io.ReadAll(resp.Body)
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatal(err)
				}
				if err = resp.Body.Close(); err != nil {
					t.Fatal(err)
				}
				if s := string(b); s != "too short" {
					t.Fatal(s)
				}
			})
		})
	})

	t.Run("Unwrap", func(t *testing.T) {
		var r http.RoundTripper = &roundtrippers.Log{Transport: http.DefaultTransport}
		u, ok := r.(roundtrippers.Unwrapper)
		if !ok || u.Unwrap() != http.DefaultTransport {
			t.Fatal("unexpected")
		}
	})
}
