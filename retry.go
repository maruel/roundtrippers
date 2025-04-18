// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

// Retry retries a request on HTTP 429 or 5xx.
type Retry struct {
	Transport http.RoundTripper
	// Policy determines if an HTTP request should be retried and after how much time.
	//
	// If unset, defaults to DefaultRetryPolicy.
	Policy RetryPolicy
}

// RoundTrip implements http.RoundTripper.
func (r *Retry) RoundTrip(req *http.Request) (*http.Response, error) {
	policy := r.Policy
	if policy == nil {
		policy = &DefaultRetryPolicy
	}
	start := time.Now()
	var in []byte
	if req.Body != nil && req.GetBody == nil {
		// See https://github.com/golang/go/issues/73439
		var err error
		if in, err = io.ReadAll(req.Body); err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewBuffer(in))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBuffer(in)), nil
		}
	}
	resp, err := r.Transport.RoundTrip(req)
	ctx := req.Context()
	for try := 0; policy.ShouldRetry(ctx, start, try, err, resp); try++ {
		if req.Body != nil {
			if req.GetBody != nil {
				var err2 error
				if req.Body, err2 = req.GetBody(); err2 != nil {
					return resp, err2
				}
			} else {
				// See https://github.com/golang/go/issues/73439
				req.Body = io.NopCloser(bytes.NewBuffer(in))
			}
		}
		// "Retry-After" is generally sent along HTTP 429. If the server then this header, use this instead of our
		// backoff algorithm.
		sleep, ok := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
		if !ok {
			sleep = policy.Backoff(start, try)
		}
		select {
		case <-ctx.Done():
			// Return the previous try response untouched.
			return resp, err
		case <-timeAfter(sleep):
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		resp, err = r.Transport.RoundTrip(req)
	}
	return resp, err
}

// Unwrap implements Unwrapper.
func (t *Retry) Unwrap() http.RoundTripper {
	return t.Transport
}

// RetryPolicy determines when Retry should retry an HTTP request.
type RetryPolicy interface {
	ShouldRetry(ctx context.Context, start time.Time, try int, err error, resp *http.Response) bool
	Backoff(start time.Time, try int) time.Duration
}

// ExponentialBackoff uses exponential backoff.
type ExponentialBackoff struct {
	MaxTryCount int
	MaxDuration time.Duration
	Exp         float64
}

// DefaultRetryPolicy is a reasonable default policy.
var DefaultRetryPolicy = ExponentialBackoff{
	MaxTryCount: 3,
	MaxDuration: 10 * time.Second, // 2^0 + 2^1 + 2^2 == 7
	Exp:         2,
}

func (e *ExponentialBackoff) ShouldRetry(ctx context.Context, start time.Time, try int, err error, resp *http.Response) bool {
	if try >= e.MaxTryCount || time.Since(start) > e.MaxDuration || ctx.Err() != nil || !isRetriableError(err) || resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusTooManyRequests || // 429
		resp.StatusCode == http.StatusBadGateway || // 502
		resp.StatusCode == http.StatusServiceUnavailable || // 503
		resp.StatusCode == http.StatusGatewayTimeout // 504
}

func (e *ExponentialBackoff) Backoff(start time.Time, try int) time.Duration {
	return time.Duration(math.Pow(e.Exp, float64(try))) * time.Second
}

//

var (
	// redirectsErrorRe matches the error returned by net/http when the configured number of redirects is
	// exhausted. This error isn't typed specifically so we resort to matching on the error string.
	redirectsErrorRe = regexp.MustCompile(`stopped after \d+ redirects\z`)
	// schemeErrorRe matches the error returned by net/http when the scheme specified in the URL is invalid.
	// This error isn't typed specifically so we resort to matching on the error string.
	schemeErrorRe = regexp.MustCompile(`unsupported protocol scheme`)
	// invalidHeaderErrorRe matches the error returned by net/http when a request header or value is invalid.
	// This error isn't typed specifically so we resort to matching on the error string.
	invalidHeaderErrorRe = regexp.MustCompile(`invalid header`)
	// notTrustedErrorRe matches the error returned by net/http when the TLS certificate is not trusted. This
	// error isn't typed specifically so we resort to matching on the error string.
	notTrustedErrorRe = regexp.MustCompile(`certificate is not trusted`)
)

var timeAfter = time.After

func parseRetryAfterHeader(header string) (time.Duration, bool) {
	if sleep, err := strconv.ParseInt(header, 10, 64); err == nil {
		if sleep > 0 {
			return time.Second * time.Duration(sleep), true
		}
	} else if retryTime, err := time.Parse(time.RFC1123, header); err == nil {
		if until := time.Until(retryTime); until > 0 {
			return until, true
		}
	}
	return 0, false
}

func isRetriableError(err error) bool {
	if v, ok := err.(*url.Error); ok {
		if _, ok := v.Err.(*tls.CertificateVerificationError); ok {
			return false
		}
		if s := v.Error(); redirectsErrorRe.MatchString(s) || schemeErrorRe.MatchString(s) || invalidHeaderErrorRe.MatchString(s) || notTrustedErrorRe.MatchString(s) {
			return false
		}
	}
	return true
}
