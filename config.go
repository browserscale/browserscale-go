package browserscale

// BrowserConfig holds all parameters for renting a browser session.
// Use NewBrowserConfig with the required fields, then chain optional setters.
type BrowserConfig struct {
	apiKey          string
	rentDuration    int
	proxyHost       string
	proxyPort       int
	proxyUsername   string
	proxyPassword   string
	countryCode     string
	timezone        string
	fingerprint     string
	webglRenderer   string
	webglVendor     string
	webglExtensions []string
}

// NewBrowserConfig returns a [BrowserConfig] populated with the required
// rental fields. Optional fields are configured via the chainable With…
// setters before passing the config to [RentBrowser].
//
// @param apiKey - API key authenticating the rental
// @param rentDuration - lifetime of the session in seconds
// @param proxyHost - upstream proxy host (empty string disables the proxy)
// @param proxyPort - upstream proxy port (ignored when proxyHost is empty)
// @param proxyUsername - proxy auth user (empty for unauthenticated proxies)
// @param proxyPassword - proxy auth password (empty for unauthenticated proxies)
//
// @returns *BrowserConfig ready to be customized further or passed to RentBrowser
//
// @example
//
//	cfg := browserscale.NewBrowserConfig("sk_…", 600, "", 0, "", "").
//	    WithCountryCode("DE").
//	    WithTimezone("Europe/Berlin")
func NewBrowserConfig(apiKey string, rentDuration int, proxyHost string, proxyPort int, proxyUsername string, proxyPassword string) *BrowserConfig {
	return &BrowserConfig{
		apiKey:        apiKey,
		rentDuration:  rentDuration,
		proxyHost:     proxyHost,
		proxyPort:     proxyPort,
		proxyUsername: proxyUsername,
		proxyPassword: proxyPassword,
	}
}

// WithCountryCode sets the geo-IP country code for the rented session.
//
// Drives both the assigned exit-IP region and the locale defaults (Accept-
// Language, timezone fallback) when those are not overridden separately.
//
// @param countryCode - ISO-3166 country code (e.g. "DE", "US")
//
// @returns the modified *BrowserConfig for chaining
//
// @example
//
//	cfg := browserscale.NewBrowserConfig(apiKey, 600, "", 0, "", "").WithCountryCode("DE")
func (c *BrowserConfig) WithCountryCode(countryCode string) *BrowserConfig {
	c.countryCode = countryCode
	return c
}

// WithTimezone sets the IANA timezone for the rented session.
//
// @param timezone - IANA timezone (e.g. "Europe/Berlin")
//
// @returns the modified *BrowserConfig for chaining
//
// @example
//
//	cfg := browserscale.NewBrowserConfig(apiKey, 600, "", 0, "", "").WithTimezone("Europe/Berlin")
func (c *BrowserConfig) WithTimezone(timezone string) *BrowserConfig {
	c.timezone = timezone
	return c
}

// WithFingerprint pins a specific browser fingerprint id for the session.
//
// When omitted the server picks a fingerprint based on the country code.
// Pass a known id (e.g. one returned by a previous rental) to keep
// fingerprints stable across sessions.
//
// @param fingerprint - server-side fingerprint id
//
// @returns the modified *BrowserConfig for chaining
//
// @example
//
//	cfg := browserscale.NewBrowserConfig(apiKey, 600, "", 0, "", "").WithFingerprint("fp_abc123")
func (c *BrowserConfig) WithFingerprint(fingerprint string) *BrowserConfig {
	c.fingerprint = fingerprint
	return c
}

// UnstableWithFakeGpu overrides WebGL UNMASKED_RENDERER_WEBGL,
// UNMASKED_VENDOR_WEBGL and getSupportedExtensions().
//
// Unstable API — likely to be reshaped or removed without notice. Use only
// when you have a specific WebGL-fingerprint requirement.
//
// @param renderer - value to return for UNMASKED_RENDERER_WEBGL
// @param vendor - value to return for UNMASKED_VENDOR_WEBGL
// @param extensions - list returned by getSupportedExtensions()
//
// @returns the modified *BrowserConfig for chaining
//
// @example
//
//	cfg.UnstableWithFakeGpu("ANGLE", "Google Inc.", []string{"OES_texture_float"})
func (c *BrowserConfig) UnstableWithFakeGpu(renderer string, vendor string, extensions []string) *BrowserConfig {
	c.webglRenderer = renderer
	c.webglVendor = vendor
	c.webglExtensions = extensions
	return c
}
