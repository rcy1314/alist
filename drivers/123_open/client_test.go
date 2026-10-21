package _123Open

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pan123 "github.com/okatu-loli/go-123pan"
)

func TestNewSDKClientAuthModes(t *testing.T) {
	tests := []struct {
		name    string
		add     Addition
		wantErr string
	}{
		{
			name: "client credentials",
			add:  Addition{AuthMode: AuthClientCredentials, ClientID: "id", ClientSecret: "secret"},
		},
		{
			name: "empty auth mode defaults to client credentials",
			add:  Addition{ClientID: "id", ClientSecret: "secret"},
		},
		{
			name:    "client credentials without secret",
			add:     Addition{AuthMode: AuthClientCredentials, ClientID: "id"},
			wantErr: "clientID and clientSecret are required",
		},
		{
			name: "token from an onboarded enterprise",
			add:  Addition{AuthMode: AuthToken, AccessToken: "at"},
		},
		{
			name:    "token mode without token",
			add:     Addition{AuthMode: AuthToken},
			wantErr: "access_token is required",
		},
		{
			name:    "unknown mode",
			add:     Addition{AuthMode: "nope"},
			wantErr: "unknown auth_mode",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Open123{Addition: tt.add}
			c, err := d.newSDKClient()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c == nil {
				t.Fatal("client is nil")
			}
			if tt.add.AuthMode == AuthToken && c.Token() != tt.add.AccessToken {
				t.Fatalf("token = %q, want %q", c.Token(), tt.add.AccessToken)
			}
		})
	}
}

// In token mode the platform rotates the refresh_token on every refresh and
// voids the previous one, so a renewed pair must replace the configured values.
func TestEnsureTokenRotatesCredentials(t *testing.T) {
	var gotGrant, gotRefresh, gotClientID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/oauth2/access_token" {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotGrant = r.URL.Query().Get("grant_type")
		gotRefresh = r.URL.Query().Get("refresh_token")
		gotClientID = r.URL.Query().Get("client_id")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token_type":    "Bearer",
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    7200,
		})
	}))
	defer srv.Close()

	d := &Open123{Addition: Addition{
		AuthMode:     AuthToken,
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ClientID:     "cid",
		ClientSecret: "csecret",
	}}
	d.client = pan123.NewWithToken("old-access", pan123.WithBaseURL(srv.URL))
	// mark the token as already expired so a refresh is due
	d.accessTokenExpiredAt = time.Now().Add(-time.Minute)
	var persisted int
	d.persistFn = func() { persisted++ }

	if err := d.ensureToken(context.Background()); err != nil {
		t.Fatalf("ensureToken: %v", err)
	}
	if persisted != 1 {
		t.Fatalf("storage persisted %d times, want exactly 1 (the old refresh_token is already void)", persisted)
	}
	if gotGrant != "refresh_token" || gotRefresh != "old-refresh" || gotClientID != "cid" {
		t.Fatalf("refresh request: grant=%q refresh=%q client=%q", gotGrant, gotRefresh, gotClientID)
	}
	if d.AccessToken != "new-access" {
		t.Fatalf("AccessToken = %q, want new-access", d.AccessToken)
	}
	if d.RefreshToken != "new-refresh" {
		t.Fatalf("RefreshToken = %q, want new-refresh (rotated value must replace the old one)", d.RefreshToken)
	}
	if d.client.Token() != "new-access" {
		t.Fatalf("client token = %q, want new-access", d.client.Token())
	}
	if d.accessTokenExpiredAt.Before(time.Now().Add(time.Hour)) {
		t.Fatalf("expiry not advanced: %v", d.accessTokenExpiredAt)
	}
}

func TestEnsureTokenSkipsWhenNotDue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected, got %s", r.URL.Path)
	}))
	defer srv.Close()

	// client_credentials mode: the SDK owns the token lifecycle
	d := &Open123{Addition: Addition{AuthMode: AuthClientCredentials, ClientID: "id", ClientSecret: "s"}}
	d.client = pan123.New("id", "s", pan123.WithBaseURL(srv.URL))
	if err := d.ensureToken(context.Background()); err != nil {
		t.Fatalf("client_credentials ensureToken: %v", err)
	}

	// token mode with a token that is still valid
	d2 := &Open123{Addition: Addition{AuthMode: AuthToken, AccessToken: "at", RefreshToken: "rt", ClientID: "c", ClientSecret: "s"}}
	d2.client = pan123.NewWithToken("at", pan123.WithBaseURL(srv.URL))
	d2.accessTokenExpiredAt = time.Now().Add(2 * time.Hour)
	if err := d2.ensureToken(context.Background()); err != nil {
		t.Fatalf("valid token ensureToken: %v", err)
	}

	// token mode with unknown expiry and no way to refresh: keep using the token
	d3 := &Open123{Addition: Addition{AuthMode: AuthToken, AccessToken: "at"}}
	d3.client = pan123.NewWithToken("at", pan123.WithBaseURL(srv.URL))
	if err := d3.ensureToken(context.Background()); err != nil {
		t.Fatalf("static token ensureToken: %v", err)
	}
}

func TestEnsureTokenWithoutRefreshCredentials(t *testing.T) {
	d := &Open123{Addition: Addition{AuthMode: AuthToken, AccessToken: "at"}}
	d.accessTokenExpiredAt = time.Now().Add(-time.Minute)
	err := d.ensureToken(context.Background())
	if err == nil {
		t.Fatal("expected an error when the token expired and no refresh credentials are configured")
	}
	if !contains(err.Error(), "refresh_token") {
		t.Fatalf("error = %q, want it to mention refresh_token", err)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
