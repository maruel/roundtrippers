// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Test HTTP request identifiers.

package roundtrippers_test

import (
	"net/http"
	"testing"

	"github.com/maruel/roundtrippers"
)

func TestRequestID_Unwrap(t *testing.T) {
	var r http.RoundTripper = &roundtrippers.RequestID{Transport: http.DefaultTransport}
	if r.(roundtrippers.Unwrapper).Unwrap() != http.DefaultTransport {
		t.Fatal("unexpected")
	}
}
