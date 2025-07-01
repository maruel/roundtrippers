// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/maruel/roundtrippers"
)

func TestThrottle_Unwrap(t *testing.T) {
	var r http.RoundTripper = &roundtrippers.Throttle{Transport: http.DefaultTransport}
	if r.(roundtrippers.Unwrapper).Unwrap() != http.DefaultTransport {
		t.Fatal("unexpected")
	}
}

func TestThrottle_RoundTrip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()

	var mu sync.Mutex
	var sleeps []time.Duration
	c := http.Client{
		Transport: &roundtrippers.Throttle{
			Transport: http.DefaultTransport,
			QPS:       10,
			TimeAfter: func(d time.Duration) <-chan time.Time {
				mu.Lock()
				defer mu.Unlock()
				sleeps = append(sleeps, d)
				ch := make(chan time.Time, 1)
				ch <- time.Now()
				return ch
			},
		},
	}

	// The first request should not sleep.
	// Subsequent requests should sleep for a bit.
	for i := range 3 {
		resp, err := c.Get(ts.URL)
		if err != nil {
			t.Fatalf("req %d: %v", i, err)
		}
		if _, err = io.ReadAll(resp.Body); err != nil {
			t.Fatalf("req %d: %v", i, err)
		}
		_ = resp.Body.Close()
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sleeps) != 2 {
		t.Fatalf("expected 2 sleeps, got %d: %v", len(sleeps), sleeps)
	}
	// The sleeps should be around 100ms.
	// Due to test execution speed, it can vary a bit.
	if sleeps[0] < 90*time.Millisecond {
		t.Errorf("sleep 0 is too short: %s", sleeps[0])
	}
	if sleeps[1] < 90*time.Millisecond {
		t.Errorf("sleep 1 is too short: %s", sleeps[1])
	}
}

func TestThrottle_NoThrottle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()

	var slept bool
	c := http.Client{
		Transport: &roundtrippers.Throttle{
			Transport: http.DefaultTransport,
			QPS:       0, // No throttling.
			TimeAfter: func(d time.Duration) <-chan time.Time {
				slept = true
				ch := make(chan time.Time, 1)
				ch <- time.Now()
				return ch
			},
		},
	}

	for i := range 3 {
		resp, err := c.Get(ts.URL)
		if err != nil {
			t.Fatalf("req %d: %v", i, err)
		}
		if _, err = io.ReadAll(resp.Body); err != nil {
			t.Fatalf("req %d: %v", i, err)
		}
		_ = resp.Body.Close()
	}

	if slept {
		t.Fatal("should not have slept")
	}
}

func TestThrottle_RoundTrip_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	c := http.Client{
		Transport: &roundtrippers.Throttle{
			Transport: http.DefaultTransport,
			QPS:       0.1, // 0.1 QPS, so 10 seconds per query.
			TimeAfter: func(d time.Duration) <-chan time.Time {
				// Signal that we are sleeping.
				wg.Done()
				// A channel that will never receive.
				return make(chan time.Time)
			},
		},
	}

	// First request to set the last request time.
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)

	errChan := make(chan error)
	go func() {
		resp, err := c.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		errChan <- err
	}()

	// Wait for the goroutine to start sleeping.
	wg.Wait()

	// Cancel the context.
	cancel()

	select {
	case err := <-errChan:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
