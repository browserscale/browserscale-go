package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// UaBrand is one brand/version pair of the User-Agent client hints. For Brands
// the version is the significant (major) version; for FullVersionList the full
// version.
type UaBrand struct {
	Brand   string
	Version string
}

// ProxyConfig is the session's proxy as the browser uses it. An empty Host
// means a direct connection. Masque is the HTTP/3 MASQUE proxy, if any.
type ProxyConfig struct {
	Scheme   string
	Host     string
	Port     int
	Username string
	Password string
	Masque   string
}

// ClientIdentity is everything the browser reports about itself on the wire, so
// an out-of-browser HTTP client can reproduce matching request headers: the
// User-Agent, the full UA client-hint set, the Accept-Language, and the
// session's proxy. Cookies and Storage are populated only when requested via
// WithCookies / WithStorage.
type ClientIdentity struct {
	UserAgent       string
	AcceptLanguage  string
	Platform        string
	PlatformVersion string
	Architecture    string
	Bitness         string
	Model           string
	Mobile          bool
	Wow64           bool
	Brands          []UaBrand
	FullVersionList []UaBrand
	Proxy           ProxyConfig

	// Cookies is set when GetClientIdentity is called WithCookies().
	Cookies []CookieParam
	// Storage is set when GetClientIdentity is called WithStorage().
	Storage []StorageOriginEntry
}

type identityConfig struct {
	includeCookies bool
	includeStorage bool
	storageOrigin  string
}

// IdentityOption customizes what GetClientIdentity folds into one call.
type IdentityOption func(*identityConfig)

// WithCookies also returns the session's cookies in ClientIdentity.Cookies.
func WithCookies() IdentityOption {
	return func(c *identityConfig) { c.includeCookies = true }
}

// WithStorage also returns the session's localStorage in ClientIdentity.Storage.
func WithStorage() IdentityOption {
	return func(c *identityConfig) { c.includeStorage = true }
}

// WithStorageOrigin restricts WithStorage to a single origin (and enables it),
// e.g. "https://example.com".
func WithStorageOrigin(origin string) IdentityOption {
	return func(c *identityConfig) {
		c.includeStorage = true
		c.storageOrigin = origin
	}
}

// GetClientIdentity returns the session's client identity in one call, and —
// with WithCookies / WithStorage — its cookies and localStorage folded into the
// same response. Reads the context configuration directly in the browser
// process, so no page needs to be open.
//
// @param opts - optional WithCookies / WithStorage / WithStorageOrigin
//
// @returns *ClientIdentity describing the session on the wire
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	id, err := browser.GetClientIdentity(ctx, browserscale.WithCookies())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(id.UserAgent, id.Proxy.Host, len(id.Cookies))
func (c *CloudBrowser) GetClientIdentity(ctx context.Context, opts ...IdentityOption) (*ClientIdentity, error) {
	var cfg identityConfig
	for _, o := range opts {
		o(&cfg)
	}
	req := &generated.GetClientIdentityRequest{SessionId: c.sessionId, ApiKey: c.apiKey}
	if cfg.includeCookies {
		v := true
		req.IncludeCookies = &v
	}
	if cfg.includeStorage {
		v := true
		req.IncludeStorage = &v
	}
	if cfg.storageOrigin != "" {
		s := cfg.storageOrigin
		req.StorageOrigin = &s
	}

	resp, err := c.client.GetClientIdentity(ctx, req)
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("getClientIdentity", resp.GetError()); e != nil {
		return nil, e
	}
	return clientIdentityFromProto(resp), nil
}

func clientIdentityFromProto(resp *generated.GetClientIdentityResponse) *ClientIdentity {
	id := &ClientIdentity{
		Cookies: cookiesFromProto(resp.Cookies),
		Storage: storageFromProto(resp.Storage),
	}
	if p := resp.Identity; p != nil {
		id.UserAgent = p.UserAgent
		id.AcceptLanguage = p.AcceptLanguage
		id.Platform = p.Platform
		id.PlatformVersion = p.PlatformVersion
		id.Architecture = p.Architecture
		id.Bitness = p.Bitness
		id.Model = p.Model
		id.Mobile = p.Mobile
		id.Wow64 = p.Wow64
		id.Brands = brandsFromProto(p.Brands)
		id.FullVersionList = brandsFromProto(p.FullVersionList)
		if pr := p.Proxy; pr != nil {
			id.Proxy = ProxyConfig{
				Scheme:   pr.Scheme,
				Host:     pr.Host,
				Port:     int(pr.Port),
				Username: pr.Username,
				Password: pr.Password,
				Masque:   pr.Masque,
			}
		}
	}
	return id
}

func brandsFromProto(in []*generated.UaBrand) []UaBrand {
	if len(in) == 0 {
		return nil
	}
	out := make([]UaBrand, len(in))
	for i, b := range in {
		out[i] = UaBrand{Brand: b.Brand, Version: b.Version}
	}
	return out
}
