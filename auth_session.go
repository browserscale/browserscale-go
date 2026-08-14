package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// DbscSession is one Device Bound Session Credentials entry.
type DbscSession struct {
	// Site is the serialized schemeful site key, e.g. "https://google.com".
	Site string
	// Session is base64 of the serialized DBSC Session proto (includes the
	// wrapped binding key; portable under WRC's software key provider).
	Session string
}

// AuthSession is a portable snapshot of a context's signed-in Google account
// and/or DBSC sessions. All fields are optional so a context that only has
// DBSC (no primary account) or only a sign-in (no DBSC) round-trips.
//
// Pair with [CloudBrowser.GetCookies] / [CloudBrowser.SetCookies] and
// [CloudBrowser.GetStorage] / [CloudBrowser.SetStorage] to fully move a persona
// between fresh contexts. Call [CloudBrowser.SetAuthSession] before navigating.
type AuthSession struct {
	GaiaID               *string
	Email                *string
	RefreshToken         *string
	WrappedBindingKey    *string
	SigninScopedDeviceID *string
	SyncConsent          *bool
	DbscSessions         []DbscSession
}

// GetAuthSession exports the signed-in primary account and DBSC sessions of
// this browser context. Reads state in the browser process — no page needed.
//
// Returns nil, nil when the context has neither a signed-in account nor DBSC
// sessions.
//
// @returns *AuthSession, or nil when there is nothing to export
//
// @throws UNKNOWN_ERROR - the auth session could not be read
//
// @example
//
//	auth, err := browser.GetAuthSession(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if auth == nil {
//	    log.Println("no auth/DBSC state")
//	    return
//	}
//	// persist auth, then later SetAuthSession on a fresh rent
func (c *CloudBrowser) GetAuthSession(ctx context.Context) (*AuthSession, error) {
	resp, err := c.client.GetAuthSession(ctx, &generated.GetAuthSessionRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	return authSessionFromProto(resp.Session), nil
}

// SetAuthSession imports an auth session so the context comes up signed in
// (and syncing if SyncConsent) with its DBSC sessions restored.
//
// Call before navigating. Pair with SetCookies / SetStorage to fully restore
// a persona.
//
// @param session - session as returned by GetAuthSession
//
// @throws UNKNOWN_ERROR - the auth session could not be written
//
// @example
//
//	_ = browser.SetAuthSession(ctx, *saved)
//	_ = browser.Navigate(ctx, "https://mail.google.com")
func (c *CloudBrowser) SetAuthSession(ctx context.Context, session AuthSession) error {
	_, err := c.client.SetAuthSession(ctx, &generated.SetAuthSessionRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Session: authSessionToProto(&session),
	})
	return err
}
