package gowebdav

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientRetriesStaleDigestChallenge(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch requests.Add(1) {
		case 1:
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("initial request unexpectedly had authorization: %q", got)
			}
			w.Header().Set("WWW-Authenticate", `Digest realm="webdav", nonce="nonce-one", algorithm=MD5, qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
		case 2:
			if got := r.Header.Get("Authorization"); !strings.Contains(got, `nonce="nonce-one"`) {
				t.Errorf("first digest request used the wrong nonce: %q", got)
			}
			w.Header().Set("WWW-Authenticate", `Digest realm="webdav", nonce="nonce-two", algorithm=MD5, qop="auth", stale=true`)
			w.WriteHeader(http.StatusUnauthorized)
		case 3:
			if got := r.Header.Get("Authorization"); !strings.Contains(got, `nonce="nonce-two"`) {
				t.Errorf("retried digest request used the wrong nonce: %q", got)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request %d", requests.Load())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "password")
	if err := client.Connect(); err != nil {
		t.Fatalf("connect with a refreshed digest nonce: %v", err)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("request count = %d, want 3", got)
	}
}

func TestClientLimitsStaleDigestRetries(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := requests.Add(1)
		nonce := "nonce-one"
		if request > 1 {
			nonce = "nonce-two"
		}
		w.Header().Set("WWW-Authenticate", `Digest realm="webdav", nonce="`+nonce+`", algorithm=MD5, qop="auth", stale=true`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "password")
	if err := client.Connect(); err == nil {
		t.Fatal("connect unexpectedly succeeded after repeated stale challenges")
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("request count = %d, want one initial request, one digest request, and one stale retry", got)
	}
}
