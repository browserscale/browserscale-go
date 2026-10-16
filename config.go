package browserscale

// BrowserConfig holds all parameters for renting a browser session.
// Use NewBrowserConfig with the required fields, then chain optional setters.
type BrowserConfig struct {
	apiKey        string
	rentDuration  int
	proxyHost     string
	proxyPort     int
	proxyUsername string
	proxyPassword string
	proxyScheme   ProxyScheme
	masqueProxy   string
	countryCode   string
	timezone      string
	fingerprint   string
	gpuEnabled    bool
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

// ProxyScheme is the protocol the browser speaks to its proxy.
type ProxyScheme string

const (
	// ProxyHTTP is an HTTP proxy (CONNECT); the default.
	ProxyHTTP ProxyScheme = "http"
	// ProxySOCKS5 is a SOCKS5 proxy. Host names are always resolved by the
	// proxy, so "socks5h" means the same and is accepted too.
	ProxySOCKS5 ProxyScheme = "socks5"
)

// WithProxyScheme sets the protocol of the proxy passed to
// [NewBrowserConfig]. Without it the proxy is an HTTP proxy.
//
// @param scheme - [ProxyHTTP] or [ProxySOCKS5]
//
// @returns the modified *BrowserConfig for chaining
//
// @example
//
//	cfg := browserscale.NewBrowserConfig(apiKey, 600, "proxy.example.com", 1080, "user", "pass").
//	    WithProxyScheme(browserscale.ProxySOCKS5)
func (c *BrowserConfig) WithProxyScheme(scheme ProxyScheme) *BrowserConfig {
	c.proxyScheme = scheme
	return c
}

// WithMasque routes the session's HTTP/3 traffic through a MASQUE proxy
// (CONNECT-UDP, RFC 9298), authenticated with the username and password of
// the proxy passed to [NewBrowserConfig].
//
// The MASQUE proxy has to leave from the same exit IP as the proxy, so the
// browser shows one address: the rental is refused when they differ. Use
// credentials that keep their IP (a sticky session) where the provider
// rotates. Without a MASQUE proxy, HTTP/3 falls back to the proxy over TCP.
//
// @param hostPort - the MASQUE proxy as "host:port"
//
// @returns the modified *BrowserConfig for chaining
//
// @example
//
//	cfg := browserscale.NewBrowserConfig(apiKey, 600, "proxy.example.com", 8080, "user", "pass").
//	    WithMasque("masque.example.com:443")
func (c *BrowserConfig) WithMasque(hostPort string) *BrowserConfig {
	c.masqueProxy = hostPort
	return c
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

// UnstableWithGpuEnabled restricts the rental to hosts that render on a
// physical GPU instead of the software renderer.
//
// Unstable API — do not build on it. It exists to compare GPU-backed hosts
// against software rendering while that rollout is in progress; once every
// host is GPU-backed the flag becomes meaningless and is removed. Note that
// it narrows the pool: the rental fails rather than falling back to a
// software-rendered host, so it can report no capacity while ordinary
// rentals still succeed.
//
// @param enabled - true to require a GPU-backed host
//
// @returns the modified *BrowserConfig for chaining
//
// @example
//
//	cfg := browserscale.NewBrowserConfig(apiKey, 600, "", 0, "", "").UnstableWithGpuEnabled(true)
func (c *BrowserConfig) UnstableWithGpuEnabled(enabled bool) *BrowserConfig {
	c.gpuEnabled = enabled
	return c
}
