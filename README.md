# RoundTrippers

Collection of [http.RoundTripper](https://pkg.go.dev/net/http#RoundTripper) to
augment your http.Client

[![Go Reference](https://pkg.go.dev/badge/github.com/maruel/roundtrippers/.svg)](https://pkg.go.dev/github.com/maruel/roundtrippers/)
[![codecov](https://codecov.io/gh/maruel/roundtrippers/graph/badge.svg?token=EMMCJD4TG4)](https://codecov.io/gh/maruel/roundtrippers)


## Features

- [AcceptCompressed](https://pkg.go.dev/github.com/maruel/roundtrippers#AcceptCompressed)
  adds support for Zstandard and Brotli for download.
- [Capture](https://pkg.go.dev/github.com/maruel/roundtrippers#Capture) sends
  all the requests to a channel for inspection.
- [Header](https://pkg.go.dev/github.com/maruel/roundtrippers#Header) adds HTTP
  headers to all requests, e.g. `User-Agent` or `Authorization`. It is very
  useful when recording with
  [go-vcr](https://pkg.go.dev/gopkg.in/dnaeon/go-vcr.v4/pkg/recorder) and you
  don't want the `Authorization` bearer to be in the replay.
- [Log](https://pkg.go.dev/github.com/maruel/roundtrippers#Log) logs all
  requests to the [slog.Logger](https://pkg.go.dev/log/slog#Logger) of your
  choice.
- [PostCompressed](https://pkg.go.dev/github.com/maruel/roundtrippers#PostCompressed)
  transparently compresses POST body.
- [RequestID](https://pkg.go.dev/github.com/maruel/roundtrippers#RequestID)
  adds a unique `X-Request-ID` to every request for logging and client-server
  side tracking.


## Usage

```go
package main 

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/maruel/roundtrippers"
)

func main() {
	// Make all HTTP request in the current program:
	// - Add a X-Request-ID for tracking both client and server side.
	// - Add logging.
	// - Accept compressed responses with zstandard and brotli, in addition to gzip.
	// - Add Authorization Bearer header.

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	const apiKey = "secret-key-that-will-not-appear-in-logs!"

	http.DefaultClient.Transport = &roundtrippers.RequestID{
		Transport: &roundtrippers.Log{
			L: logger,
			Transport: &roundtrippers.AcceptCompressed{
				Transport: &roundtrippers.Header{
					Header:    http.Header{"Authorization": []string{"Bearer " + apiKey}},
					Transport: http.DefaultTransport,
				},
			},
		},
	}

	// Now any request will be logged, authenticated and compressed.
	_, _ = http.Get("...")

	// For further compression with advanced backends (e.g. Google's), you can
	// add roundtrippers.PostCompressed to compress uploads too!
	http.DefaultClient.Transport = &roundtrippers.PostCompressed{
		Encoding:  "gzip",
		Transport: http.DefaultClient.Transport,
	}

	// Now, any POST request will be compressed too!
	_, _ = http.Post("...", "application/json", strings.NewReader("{}"))
}
```

