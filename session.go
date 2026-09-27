package browserscale

import (
	"context"
	"strings"

	"github.com/browserscale/browserscale-go/generated"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// CloudBrowser is the SDK-side handle for an active browserscale browser session.
//
// One CloudBrowser corresponds to exactly one browser context, which is
// implicitly bound to its primary page server-side. The proto's page_id
// field is currently ignored server-side, so the SDK never sets it.
type CloudBrowser struct {
	apiKey         string
	sessionId      string
	grpcUrl        string
	countryCode    string
	timezone       string
	acceptLanguage string
	fingerprint    string

	client generated.BrowserClient
	conn   interface{ Close() error }
}

// RentBrowser rents a new browser session and returns a connected handle.
//
// Calls the browserscale rent endpoint with the supplied [BrowserConfig], opens a
// gRPC connection to the assigned session host, and returns a ready-to-use
// [CloudBrowser]. On any failure the partially-rented session is best-effort
// released.
//
// @param config - rental parameters built with [NewBrowserConfig]
//
// @returns *CloudBrowser ready to drive the rented session; call
//
//	[CloudBrowser.Close] or [CloudBrowser.StopBrowser] when done
//
// @throws UNKNOWN_ERROR - the rent API rejected the request or the gRPC
//
//	connection could not be established
//
// @example
//
//	cfg := browserscale.NewBrowserConfig("sk_…", 600, "", 0, "", "")
//	browser, err := browserscale.RentBrowser(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer browser.Close()
func RentBrowser(ctx context.Context, config *BrowserConfig) (*CloudBrowser, error) {
	rentResp, err := callRentApi(config)
	if err != nil {
		return nil, err
	}

	conn, err := dialGrpc(rentResp.GrpcUrl)
	if err != nil {
		_ = callStopBrowserApi(config.apiKey, rentResp.SessionId)
		return nil, err
	}

	return &CloudBrowser{
		apiKey:         config.apiKey,
		sessionId:      rentResp.SessionId,
		grpcUrl:        rentResp.GrpcUrl,
		countryCode:    rentResp.CountryCode,
		timezone:       rentResp.Timezone,
		acceptLanguage: rentResp.AcceptLanguage,
		fingerprint:    rentResp.Fingerprint,
		client:         generated.NewBrowserClient(conn),
		conn:           conn,
	}, nil
}

// ConnectSession attaches to an already-running session via gRPC.
//
// Use this when you have a session id and gRPC URL from a previous
// [RentBrowser] (for example stored across process restarts). Unlike
// RentBrowser this does not call the rent API — the session must already
// exist server-side.
//
// @param grpcUrl - the session's gRPC endpoint as returned by [CloudBrowser.GrpcUrl],
//
//	e.g. "grpcs://api.browserscale.cloud:443" (TLS) or "grpc://host:port" (plaintext)
//
// @param apiKey - API key authorizing access to the session
// @param sessionId - id of the existing session to attach to
//
// @returns *CloudBrowser attached to the existing session; the returned
//
//	handle owns no rental, so calling Close only closes the gRPC connection
//
// @throws UNKNOWN_ERROR - the gRPC connection could not be opened
//
// @example
//
//	browser, err := browserscale.ConnectSession(ctx, "grpcs://api.browserscale.cloud:443", apiKey, sessionId)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer browser.Close()
func ConnectSession(ctx context.Context, grpcUrl string, apiKey string, sessionId string) (*CloudBrowser, error) {
	conn, err := dialGrpc(grpcUrl)
	if err != nil {
		return nil, err
	}
	return &CloudBrowser{
		apiKey:    apiKey,
		sessionId: sessionId,
		grpcUrl:   grpcUrl,
		client:    generated.NewBrowserClient(conn),
		conn:      conn,
	}, nil
}

// StopBrowser releases the session and closes the gRPC connection.
//
// Calls the browserscale stop endpoint to release the rental, then closes the
// underlying gRPC connection. Safe to call multiple times — subsequent
// calls on a closed connection return an error from the second close.
//
// @throws UNKNOWN_ERROR - the stop API or the gRPC close returned an error
//
// @see [CloudBrowser.Close] - same operation with a background context
//
// @example
//
//	defer browser.StopBrowser(context.Background())
func (c *CloudBrowser) StopBrowser(ctx context.Context) error {
	stopErr := callStopBrowserApi(c.apiKey, c.sessionId)
	closeErr := c.conn.Close()
	if stopErr != nil {
		return stopErr
	}
	return closeErr
}

// Close is the defer-friendly alias for [CloudBrowser.StopBrowser] that uses
// a background context.
//
// @inheritDoc [CloudBrowser.StopBrowser]
//
// @example
//
//	browser, err := browserscale.RentBrowser(ctx, cfg)
//	if err != nil { log.Fatal(err) }
//	defer browser.Close()
func (c *CloudBrowser) Close() error {
	return c.StopBrowser(context.Background())
}

// CloseConn closes only the gRPC connection, leaving the server-side session
// running.
//
// Use this to detach without releasing the rental — the common case when you
// attached with [ConnectSession] to act on a session owned elsewhere, or when a
// short-lived handle should not outlive its work but the session must. Contrast
// with [CloudBrowser.Close] / [CloudBrowser.StopBrowser], which also release the
// rental via the stop endpoint.
//
// @throws UNKNOWN_ERROR - the gRPC connection could not be closed
//
// @example
//
//	browser, err := browserscale.ConnectSession(ctx, grpcUrl, apiKey, sessionId)
//	if err != nil { log.Fatal(err) }
//	defer browser.CloseConn() // detach; the session keeps running
func (c *CloudBrowser) CloseConn() error {
	return c.conn.Close()
}

// StopBrowser releases a session without needing a [CloudBrowser] handle.
//
// Useful when a session id was persisted across processes and the rental
// outlived the original handle. Only calls the rent stop endpoint; there
// is no gRPC connection to close in this form.
//
// @param apiKey - API key the session was rented with
// @param sessionId - id of the session to release
//
// @throws UNKNOWN_ERROR - the stop API rejected the request
//
// @example
//
//	_ = browserscale.StopBrowser(context.Background(), apiKey, sessionId)
func StopBrowser(ctx context.Context, apiKey string, sessionId string) error {
	return callStopBrowserApi(apiKey, sessionId)
}

// StopAllBrowsers releases every session an API key holds.
//
// The blunt instrument, for cleaning up after a run that leaked sessions — a
// crashed worker pool, an interrupted test. It ends sessions this process never
// created, including ones another machine is using, so it is not a way to tidy
// up "my" sessions in a shared account.
//
// Unused credits are refunded per session, as with [StopBrowser].
//
// @param apiKey - API key whose sessions to release
//
// @returns int how many sessions were stopped
//
// @throws UNKNOWN_ERROR - the stop API rejected the request
//
// @example
//
//	stopped, err := browserscale.StopAllBrowsers(ctx, apiKey)
func StopAllBrowsers(ctx context.Context, apiKey string) (int, error) {
	return callStopAllApi(apiKey)
}

// BrowserInfo describes one running session as [ListBrowsers] reports it.
type BrowserInfo struct {
	SessionId string `json:"sessionId"`
	// GrpcUrl is the endpoint this session is driven from — the same one rent
	// returned. It is what makes a listed id usable: pass it to
	// [ConnectSession], or call [BrowserInfo.Connect].
	GrpcUrl string `json:"grpcUrl"`
	// StartTime is unix seconds.
	StartTime int64 `json:"startTime"`
	// RentDuration is the rental length in seconds; 0 means unlimited.
	RentDuration int `json:"rentDuration"`
	// RemainingSeconds counts down to the end of the rental, and is nil for an
	// unlimited one.
	RemainingSeconds *int   `json:"remainingSeconds"`
	CountryCode      string `json:"countryCode"`
	Timezone         string `json:"timezone"`
	ProxyHost        string `json:"proxyHost"`
	// PublicIp is the address the session egresses from.
	PublicIp string `json:"publicIp"`
	// GpuIndex is the physical card the session renders on, nil on a
	// software-rendered host.
	GpuIndex *int `json:"gpuIndex"`
}

// ListBrowsers reports the sessions an API key currently holds.
//
// Use it to recover session ids the process lost — after a restart, or from a
// different machine entirely. Without it a rental is only reachable through the
// handle that created it, so a crash between rent and stop leaves a paid session
// running with nothing able to name it.
//
// Only live sessions are listed; a stopped one is gone, not reported as ended.
//
// @param apiKey - API key whose sessions to list
//
// @returns []BrowserInfo oldest first, empty when the key holds none
//
// @throws UNKNOWN_ERROR - the list API rejected the request
//
// @example
//
//	browsers, err := browserscale.ListBrowsers(ctx, apiKey)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, b := range browsers {
//	    fmt.Println(b.SessionId, b.CountryCode)
//	}
func ListBrowsers(ctx context.Context, apiKey string) ([]BrowserInfo, error) {
	return callListSessionsApi(apiKey)
}

// Connect attaches to this listed session over gRPC.
//
// Shorthand for [ConnectSession] with the URL and id already in hand. The
// returned handle owns no rental, so [CloudBrowser.CloseConn] detaches without
// ending the session — which is usually what you want for a session you found
// rather than rented.
//
// @param apiKey - API key the session was rented with
//
// @returns *CloudBrowser attached to the session
//
// @throws UNKNOWN_ERROR - the gRPC connection could not be opened
//
// @example
//
//	browsers, _ := browserscale.ListBrowsers(ctx, apiKey)
//	browser, err := browsers[0].Connect(ctx, apiKey)
//	if err != nil { log.Fatal(err) }
//	defer browser.CloseConn() // detach; the session keeps running
func (b BrowserInfo) Connect(ctx context.Context, apiKey string) (*CloudBrowser, error) {
	return ConnectSession(ctx, b.GrpcUrl, apiKey, b.SessionId)
}

// dialGrpc opens a gRPC client connection, choosing transport security from
// the URL scheme: "grpcs://" dials with TLS against the system root CAs,
// "grpc://" (or no scheme, for older servers) dials plaintext.
func dialGrpc(grpcUrl string) (*grpc.ClientConn, error) {
	target := grpcUrl
	creds := insecure.NewCredentials()
	switch {
	case strings.HasPrefix(grpcUrl, "grpcs://"):
		target = strings.TrimPrefix(grpcUrl, "grpcs://")
		creds = credentials.NewClientTLSFromCert(nil, "")
	case strings.HasPrefix(grpcUrl, "grpc://"):
		target = strings.TrimPrefix(grpcUrl, "grpc://")
	}
	return grpc.NewClient(target, grpc.WithTransportCredentials(creds))
}

// SetApiEndpoint overrides the HTTP rent/stop endpoint.
//
// Defaults to `https://api.browserscale.cloud`. Call this before any
// [RentBrowser]/[StopBrowser] call if you need to point at a private browserscale
// deployment.
//
// @param endpoint - base URL of the rent/stop service, with no trailing slash
//
// @example
//
//	browserscale.SetApiEndpoint("https://browserscale.internal.example.com")
func SetApiEndpoint(endpoint string) { setApiEndpoint(endpoint) }

// --- Getters ---

// SessionId returns the unique server-assigned id for this browser session.
func (c *CloudBrowser) SessionId() string { return c.sessionId }

// ApiKey returns the API key used to rent this session.
func (c *CloudBrowser) ApiKey() string { return c.apiKey }

// GrpcUrl returns the gRPC endpoint the session is connected to.
func (c *CloudBrowser) GrpcUrl() string { return c.grpcUrl }

// CountryCode returns the ISO-3166 country code the server allocated for this
// session (drives geo-IP and locale defaults).
func (c *CloudBrowser) CountryCode() string { return c.countryCode }

// Timezone returns the IANA timezone the session was provisioned with
// (e.g. "Europe/Berlin").
func (c *CloudBrowser) Timezone() string { return c.timezone }

// AcceptLanguage returns the Accept-Language header value the session was
// provisioned with.
func (c *CloudBrowser) AcceptLanguage() string { return c.acceptLanguage }

// Fingerprint returns the browser fingerprint id in use for this session.
func (c *CloudBrowser) Fingerprint() string { return c.fingerprint }
