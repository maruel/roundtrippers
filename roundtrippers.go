// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package roundtrippers is a collection of high quality http.RoundTripper to
// augment your http.Client.
package roundtrippers

import "net/http"

// Unwrapper enables users to get the underlying transport when wrapped with a middleware.
type Unwrapper interface {
	Unwrap() http.RoundTripper
}
