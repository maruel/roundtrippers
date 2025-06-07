# RoundTrippers

Collection of high quality
[http.RoundTripper](https://pkg.go.dev/net/http#RoundTripper) to augment your
http.Client.

[![Go Reference](https://pkg.go.dev/badge/github.com/maruel/roundtrippers/.svg)](https://pkg.go.dev/github.com/maruel/roundtrippers/)
[![codecov](https://codecov.io/gh/maruel/roundtrippers/graph/badge.svg?token=EMMCJD4TG4)](https://codecov.io/gh/maruel/roundtrippers)


## Features

- 🚀 [AcceptCompressed](https://pkg.go.dev/github.com/maruel/roundtrippers#AcceptCompressed)
  adds support for Zstandard and Brotli for download.
- 🚀 [PostCompressed](https://pkg.go.dev/github.com/maruel/roundtrippers#PostCompressed)
  transparently compresses POST body. Reduce your egress bandwidth. 💰
- 🔄 [Retry](https://pkg.go.dev/github.com/maruel/roundtrippers#Retry) smartly retries on HTTP 429 and 5xx,
  even on POST. It exposes a configurable backoff policy and sleeps can be nullified for fast replay tests.
- 🗒 [Header](https://pkg.go.dev/github.com/maruel/roundtrippers#Header) adds HTTP
  headers to all requests, e.g. `User-Agent` or `Authorization`. It is very
  useful when recording with
  [go-vcr](https://pkg.go.dev/gopkg.in/dnaeon/go-vcr.v4/pkg/recorder) and you
  don't want the `Authorization` bearer to be in the replay.
- 🗒 [RequestID](https://pkg.go.dev/github.com/maruel/roundtrippers#RequestID)
  adds a unique `X-Request-ID` to every request for logging and client-server
  side tracking.
- 🧐 [Capture](https://pkg.go.dev/github.com/maruel/roundtrippers#Capture) sends
  all the requests to a channel for inspection.
- 🧐 [Log](https://pkg.go.dev/github.com/maruel/roundtrippers#Log) logs all
  requests to the [slog.Logger](https://pkg.go.dev/log/slog#Logger) of your
  choice.


## Usage

### Baseline

Make all HTTP request in the current program:
- Add a `X-Request-ID` for tracking both client and server side.
- Add logging to slog.
- Accept compressed responses with zstandard and brotli, in addition to gzip.
- Add Authorization Bearer header that is **never logged**.

Try this example in the [Go Playground](https://go.dev/play/p/rjcHtNNoHCa) ✨

```go
package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/maruel/roundtrippers"
)

func main() {
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

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	const apiKey = "secret-key-that-will-not-appear-in-logs!"

	http.DefaultClient.Transport = &roundtrippers.RequestID{
		Transport: &roundtrippers.AcceptCompressed{
			Transport: &roundtrippers.Log{
				L: logger,
				Transport: &roundtrippers.Header{
					Header:    http.Header{"Authorization": []string{"Bearer " + apiKey}},
					Transport: http.DefaultTransport,
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
```

### Compressed POST

Save on egress bandwidth! 💰

Similar to the previous example with the added twist of compressed POST! This
is useful for advanced web servers supporting compressed POST (e.g. Google's)
to save on egress bandwidth.

Try this example in the [Go Playground](https://go.dev/play/p/zDt9UFObWom) ✨

```go
package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/maruel/roundtrippers"
)

func main() {
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

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	const apiKey = "secret-key-that-will-not-appear-in-logs!"

	// Now any request will be logged, authenticated and compressed, including POST request.
	http.DefaultClient.Transport = &roundtrippers.RequestID{
		Transport: &roundtrippers.PostCompressed{
			Encoding: "gzip",
			Transport: &roundtrippers.AcceptCompressed{
				Transport: &roundtrippers.Log{
					L: logger,
					Transport: &roundtrippers.Header{
						Header:    http.Header{"Authorization": []string{"Bearer " + apiKey}},
						Transport: http.DefaultTransport,
					},
				},
			},
		},
	}

	// Now, any POST request will be compressed too!
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
```
