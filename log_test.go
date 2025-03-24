// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maruel/roundtrippers"
)

func TestLog_RoundTrip_error_bad_url(t *testing.T) {
	c := http.Client{Transport: &roundtrippers.RequestID{Transport: &roundtrippers.Log{Transport: http.DefaultTransport, L: slog.New(slog.DiscardHandler)}}}
	resp, err := c.Get("")
	if resp != nil || err == nil {
		t.Fatal(resp, err)
	}
}

func TestLog_RoundTrip_error_missing_request_id(t *testing.T) {
	c := http.Client{Transport: &roundtrippers.Log{Transport: http.DefaultTransport, L: slog.New(slog.DiscardHandler)}}
	resp, err := c.Get("")
	if resp != nil || err == nil {
		t.Fatal(resp, err)
	}
}

func TestLog_RoundTrip_error_short(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("too short"))
	}))
	defer ts.Close()
	c := http.Client{Transport: &roundtrippers.RequestID{Transport: &roundtrippers.Log{Transport: http.DefaultTransport, L: slog.New(slog.DiscardHandler)}}}
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
}

func TestLog_Unwrap(t *testing.T) {
	var r http.RoundTripper = &roundtrippers.Log{Transport: http.DefaultTransport}
	if r.(roundtrippers.Unwrapper).Unwrap() != http.DefaultTransport {
		t.Fatal("unexpected")
	}
}
