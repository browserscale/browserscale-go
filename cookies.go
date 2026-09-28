package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// CookiePartitionKey describes CHIPS partitioning metadata for partitioned cookies.
type CookiePartitionKey struct {
	TopLevelSite         string
	HasCrossSiteAncestor bool
}

// CookieParam is one cookie returned by GetCookies / passed to SetCookies.
// Name, Value, Domain, and Path are the common required identity fields;
// optional attributes mirror the browser's CookieParam shape:
// URL, Secure, HTTPOnly, SameSite, Expires, Priority, SourceScheme,
// SourcePort, and PartitionKey.
type CookieParam struct {
	Name         string
	Value        string
	URL          *string
	Domain       string
	Path         string
	Secure       *bool
	HTTPOnly     *bool
	SameSite     *string
	Expires      *float64
	Priority     *string
	SourceScheme *string
	SourcePort   *int
	PartitionKey *CookiePartitionKey
}

// GetCookies returns all cookies currently stored in this session's
// browser context.
//
// @returns []CookieParam, one per cookie in the context
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	cookies, err := browser.GetCookies(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, c := range cookies {
//	    fmt.Println(c.Name, "=", c.Value)
//	}
func (c *CloudBrowser) GetCookies(ctx context.Context) ([]CookieParam, error) {
	resp, err := c.client.GetCookies(ctx, &generated.GetCookiesRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("getCookies", resp.GetError()); e != nil {
		return nil, e
	}
	return cookiesFromProto(resp.Cookies), nil
}

// SetCookies writes the supplied cookies into the browser context.
//
// Existing cookies with the same (name, domain, path) tuple are
// overwritten. Pass an empty slice for a no-op.
//
// @param cookies - cookies to write; empty slice is a no-op
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	secure := true
//	httpOnly := true
//	sameSite := "Lax"
//
//	_ = browser.SetCookies(ctx, []browserscale.CookieParam{
//	    {
//	        Name:     "auth",
//	        Value:    "tok",
//	        Domain:   "example.com",
//	        Path:     "/",
//	        Secure:   &secure,
//	        HTTPOnly: &httpOnly,
//	        SameSite: &sameSite,
//	    },
//	})
func (c *CloudBrowser) SetCookies(ctx context.Context, cookies []CookieParam) error {
	resp, err := c.client.SetCookies(ctx, &generated.SetCookiesRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Cookies: cookiesToProto(cookies),
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("setCookies", resp.GetError())
}

// ClearCookies deletes every cookie in the browser context.
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	_ = browser.ClearCookies(ctx)
func (c *CloudBrowser) ClearCookies(ctx context.Context) error {
	resp, err := c.client.ClearCookies(ctx, &generated.ClearCookiesRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("clearCookies", resp.GetError())
}
