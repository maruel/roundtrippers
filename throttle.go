// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"net/http"
	"sync"
	"time"
)

// We could implement leaky bucket, token bucket sliding window or better. These are useful as a server but
// this is for a client.

// Throttle implements a minimalistic time based algorithm to smooth out HTTP requests at exactly QPS or less.
//
// This is meant for use as a client to make sure the access is strictly limited to never trigger a rate
// limiter on the server. As such, it doesn't have allowance for bursty requests; this is intentionally not a
// rate limiter.
type Throttle struct {
	Transport http.RoundTripper
	QPS       float64
	// TimeAfter can be hooked for unit tests to disable sleeping. It defaults to time.After().
	TimeAfter func(d time.Duration) <-chan time.Time

	mu          sync.Mutex
	lastRequest time.Time
}

func (t *Throttle) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.QPS <= 0 {
		return t.Transport.RoundTrip(req)
	}
	var sleep time.Duration
	window := time.Duration(float64(time.Second) / t.QPS)

	t.mu.Lock()
	now := time.Now()
	if !t.lastRequest.IsZero() {
		if elapsed := now.Sub(t.lastRequest); elapsed < window {
			sleep = window - elapsed
		}
	}
	t.lastRequest = now.Add(sleep)
	t.mu.Unlock()

	if sleep > 0 {
		ctx := req.Context()
		timeAfter := t.TimeAfter
		if timeAfter == nil {
			timeAfter = time.After
		}
		select {
		case <-timeAfter(sleep):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return t.Transport.RoundTrip(req)
}

func (t *Throttle) Unwrap() http.RoundTripper {
	return t.Transport
}
