// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
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
	var err error
	if req, err = cloneRequestWithBody(req); err != nil {
		return nil, err
	}
	resp, err := r.Transport.RoundTrip(req)
	ctx := req.Context()
	for try := 0; policy.ShouldRetry(ctx, start, try, err, resp); try++ {
		if req.GetBody != nil {
			var err2 error
			if req.Body, err2 = req.GetBody(); err2 != nil {
				return resp, err2
			}
		}
		var sleep time.Duration
		if resp != nil {
			// "Retry-After" is generally sent along HTTP 429. If the server then this header, use this instead of our
			// backoff algorithm.
			ok := false
			if sleep, ok = parseRetryAfterHeader(resp.Header.Get("Retry-After")); !ok {
				sleep = policy.Backoff(start, try)
			}
		} else {
			sleep = policy.Backoff(start, try)
		}
		select {
		case <-ctx.Done():
			// Return the previous try response untouched.
			return resp, err
		case <-timeAfter(sleep):
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
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
	if try >= e.MaxTryCount || time.Since(start) > e.MaxDuration || ctx.Err() != nil || isNotRetriableError(err) {
		return false
	}
	if resp == nil {
		/* TODO
		// Seems to happen often with Google frontend.
		if err != nil && http2StreamError.MatchString(err.Error()) {
			return true
		}
		*/
		return false
	}
	code := resp.StatusCode
	return code == http.StatusTooManyRequests || // 429
		code == http.StatusBadGateway || // 502
		code == http.StatusServiceUnavailable || // 503
		code == http.StatusGatewayTimeout || // 504
		code == 529 // Non-standard code. See https://http.dev/529
}

func (e *ExponentialBackoff) Backoff(start time.Time, try int) time.Duration {
	return time.Duration(math.Pow(e.Exp, float64(try))) * time.Second
}

//

// List of regexes used to match errors returned by net/http. These are not typed specifically so we resort to
// matching on the error string. This is not ideal.
var (
	// redirectsErrorRe matches the error returned by net/http when the configured number of redirects is
	// exhausted.
	redirectsErrorRe = regexp.MustCompile(`stopped after \d+ redirects\z`)
	// schemeErrorRe matches the error returned by net/http when the scheme specified in the URL is invalid.
	schemeErrorRe = regexp.MustCompile(`unsupported protocol scheme`)
	// invalidHeaderErrorRe matches the error returned by net/http when a request header or value is invalid.
	invalidHeaderErrorRe = regexp.MustCompile(`invalid header`)
	// notTrustedErrorRe matches the error returned by net/http when the TLS certificate is not trusted.
	notTrustedErrorRe = regexp.MustCompile(`certificate is not trusted`)
	/* TODO
	// http2StreamError matches the error returned by net/http when a HTTP/2 stream is closed.
	http2StreamError = regexp.MustCompile(`stream error: stream ID \d+; INTERNAL_ERROR; received from peer`)
	*/
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

// isNotRetriableError catch untyped errors that must not be retried.
func isNotRetriableError(err error) bool {
	if v, ok := err.(*url.Error); ok {
		if _, ok := v.Err.(*tls.CertificateVerificationError); ok {
			return true
		}
		if s := v.Error(); redirectsErrorRe.MatchString(s) || schemeErrorRe.MatchString(s) || invalidHeaderErrorRe.MatchString(s) || notTrustedErrorRe.MatchString(s) {
			return true
		}
	}
	return false
}
