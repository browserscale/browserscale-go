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
// @throws UNKNOWN_ERROR - the cookies could not be read
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
	return cookiesFromProto(resp.Cookies), nil
}

// SetCookies writes the supplied cookies into the browser context.
//
// Existing cookies with the same (name, domain, path) tuple are
// overwritten. Pass an empty slice for a no-op.
//
// @param cookies - cookies to write; empty slice is a no-op
//
// @throws UNKNOWN_ERROR - the cookies could not be written
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
	_, err := c.client.SetCookies(ctx, &generated.SetCookiesRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Cookies: cookiesToProto(cookies),
	})
	return err
}

// ClearCookies deletes every cookie in the browser context.
//
// @throws UNKNOWN_ERROR - the cookies could not be cleared
//
// @example
//
//	_ = browser.ClearCookies(ctx)
func (c *CloudBrowser) ClearCookies(ctx context.Context) error {
	_, err := c.client.ClearCookies(ctx, &generated.ClearCookiesRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	return err
}
