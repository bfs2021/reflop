// SPDX-License-Identifier: Apache-2.0

package selfupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// chainedRedirectServer answers /latest with `hops` redirects in sequence
// before a final 404, mirroring the github.com -> github.com/reflopai rename.
func chainedRedirectServer(t *testing.T, hops []string, seen *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(seen.Add(1)) - 1
		if n < len(hops) {
			w.Header().Set("Location", hops[n])
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestLatestFollowsMultipleRedirects(t *testing.T) {
	var seen atomic.Int32
	srv := chainedRedirectServer(t, []string{
		"/newowner/reflop/releases/latest",
		"/newowner/reflop/releases/tag/v9.9.9",
	}, &seen)
	defer srv.Close()

	u := &Updater{BaseURL: srv.URL + "/oldowner/reflop/releases", Client: srv.Client()}
	tag, err := u.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if tag != "v9.9.9" {
		t.Fatalf("tag = %q, want v9.9.9", tag)
	}
	if got := seen.Load(); got != 2 {
		t.Fatalf("followed %d requests, want 2", got)
	}
}

func TestLatestSingleRedirectStillWorks(t *testing.T) {
	var seen atomic.Int32
	srv := chainedRedirectServer(t, []string{"/reflop/releases/tag/v1.2.3"}, &seen)
	defer srv.Close()
	u := &Updater{BaseURL: srv.URL + "/releases", Client: srv.Client()}
	tag, err := u.Latest(context.Background())
	if err != nil || tag != "v1.2.3" {
		t.Fatalf("Latest = %q, %v; want v1.2.3", tag, err)
	}
}

func TestLatestRejectsCrossHostRedirect(t *testing.T) {
	var seen atomic.Int32
	srv := chainedRedirectServer(t, []string{"http://127.0.0.1:1/releases/tag/v9.9.9"}, &seen)
	defer srv.Close()
	u := &Updater{BaseURL: srv.URL + "/releases", Client: srv.Client()}
	_, err := u.Latest(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("expected a cross-host redirect error, got %v", err)
	}
}
