package _123Open

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/op"
	pan123 "github.com/okatu-loli/go-123pan"
)

// tokenRefreshMargin is how long before expiry a token is proactively renewed.
const tokenRefreshMargin = 10 * time.Minute

var errNoRefreshCredentials = errors.New("access_token expired: provide a refresh_token together with clientID/clientSecret, or switch to client_credentials mode")

// newSDKClient builds the SDK client for the configured authentication mode.
func (d *Open123) newSDKClient() (*pan123.Client, error) {
	opts := []pan123.Option{
		pan123.WithHTTPClient(&http.Client{Timeout: 60 * time.Second}),
		pan123.WithUserAgent("AList/" + conf.Version),
	}
	switch d.AuthMode {
	case AuthToken:
		if d.AccessToken == "" {
			return nil, errors.New("access_token is required in token mode")
		}
		c := pan123.NewWithToken(d.AccessToken, opts...)
		// expiry is unknown for an externally issued token; refresh on demand
		c.SetToken(d.AccessToken, d.tokenExpiry())
		return c, nil
	case AuthClientCredentials, "":
		if d.ClientID == "" || d.ClientSecret == "" {
			return nil, errors.New("clientID and clientSecret are required in client_credentials mode")
		}
		return pan123.New(d.ClientID, d.ClientSecret, opts...), nil
	default:
		return nil, fmt.Errorf("unknown auth_mode: %s", d.AuthMode)
	}
}

// tokenExpiry reports the stored expiry of an externally issued access token.
// A zero time tells the SDK never to refresh on its own; renewal is driven by
// ensureToken so the rotated refresh_token can be persisted.
func (d *Open123) tokenExpiry() time.Time {
	if d.accessTokenExpiredAt.IsZero() {
		return time.Time{}
	}
	return d.accessTokenExpiredAt
}

// ensureToken renews an externally issued access token when it is about to
// expire. The 123 open platform rotates the refresh_token on every refresh and
// invalidates the previous one, so the new pair must be written back to the
// storage config before it is used.
func (d *Open123) ensureToken(ctx context.Context) error {
	if d.AuthMode != AuthToken {
		// the SDK refreshes client_credentials tokens by itself
		return nil
	}
	d.tokenMu.Lock()
	defer d.tokenMu.Unlock()

	if !d.tokenNeedsRefresh() {
		return nil
	}
	if d.RefreshToken == "" || d.ClientID == "" || d.ClientSecret == "" {
		if d.accessTokenExpiredAt.IsZero() {
			// expiry unknown and no way to refresh: keep using the token and let
			// the API surface the authoritative error
			return nil
		}
		return errNoRefreshCredentials
	}

	token, err := d.client.OAuth.RefreshToken(ctx, d.ClientID, d.ClientSecret, d.RefreshToken)
	if err != nil {
		return fmt.Errorf("refresh access_token failed: %w", err)
	}
	d.AccessToken = token.AccessToken
	if token.RefreshToken != "" {
		d.RefreshToken = token.RefreshToken
	}
	d.accessTokenExpiredAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	d.client.SetToken(d.AccessToken, d.accessTokenExpiredAt)
	// persist immediately: the previous refresh_token is already void
	d.persist()
	return nil
}

// persist writes the rotated credentials back to the storage config.
func (d *Open123) persist() {
	if d.persistFn != nil {
		d.persistFn()
		return
	}
	op.MustSaveDriverStorage(d)
}

func (d *Open123) tokenNeedsRefresh() bool {
	if d.AccessToken == "" {
		return true
	}
	if d.accessTokenExpiredAt.IsZero() {
		return false
	}
	return time.Now().After(d.accessTokenExpiredAt.Add(-tokenRefreshMargin))
}

// tokenState is embedded in the driver; kept here so the token handling stays
// in one file.
type tokenState struct {
	tokenMu              sync.Mutex
	accessTokenExpiredAt time.Time
	// persistFn overrides how rotated credentials are saved; nil means the
	// storage is written through op. Tests set it to avoid needing a database.
	persistFn func()
}
