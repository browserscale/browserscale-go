package browserscale

import (
	"context"
	"errors"
	"io"
	"strconv"
	"sync"

	"github.com/browserscale/browserscale-go/generated"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ──────────────────────────────────────────────────────────────────────
// HeaderModification
// ──────────────────────────────────────────────────────────────────────

// HeaderModificationAction is the verb of a [HeaderModification]. Matches
// the add/edit/remove action strings; use the HeaderModificationXxx constants.
type HeaderModificationAction string

const (
	HeaderModificationAdd    HeaderModificationAction = "add"
	HeaderModificationEdit   HeaderModificationAction = "edit"
	HeaderModificationRemove HeaderModificationAction = "remove"
)

// HeaderModification is one entry passed to [CloudBrowser.ModifyRequest].
// Build it as a plain struct literal.
type HeaderModification struct {
	// Action selects what happens: HeaderModificationAdd inserts a new
	// header, HeaderModificationEdit replaces an existing header's value,
	// HeaderModificationRemove drops the header.
	Action HeaderModificationAction

	// Name is the header name the action applies to.
	Name string

	// Value is the header value for add/edit; ignored for remove.
	Value string

	// Before positions an "add" immediately before the named existing
	// header; otherwise the header is appended at the end. Ignored for
	// edit/remove.
	Before string

	// After positions an "add" immediately after the named existing
	// header. Mirror of Before; ignored for edit/remove.
	After string
}

// ──────────────────────────────────────────────────────────────────────
// RequestPattern (used by WaitForAnyRequest / WaitForAnyResponse)
// ──────────────────────────────────────────────────────────────────────

// RequestPattern matches a URL pattern in WaitForAnyRequest/Response.
// Set Abort to true to drop the request with an empty 200 response
// instead of letting it through to the network.
type RequestPattern struct {
	URL   string
	Abort bool
}

// ──────────────────────────────────────────────────────────────────────
// Commands
// ──────────────────────────────────────────────────────────────────────

// SetBlockList replaces the session's URL blocklist.
//
// Any request whose URL matches one of the supplied patterns is blocked
// before it leaves the browser. Patterns are simple URL wildcards (`*`
// matches any character span). Pass a nil/empty slice to clear the
// blocklist and let everything through.
//
// @param patterns - URL wildcards to block; nil or empty clears the list
//
// @throws UNKNOWN_ERROR - the blocklist could not be applied
//
// @example
//
//	_ = browser.SetBlockList(ctx, []string{
//	    "*.doubleclick.net/*",
//	    "*googletagmanager.com*",
//	})
func (c *CloudBrowser) SetBlockList(ctx context.Context, patterns []string) error {
	_, err := c.client.SetBlockList(ctx, &generated.SetBlockListRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Patterns: patterns,
	})
	return err
}

// SetStaticPaths configures the session to serve cached static responses
// for requests matching the given patterns from blobName.
//
// Useful for replaying frozen page assets (HTML/JS/CSS/images) without
// hitting the origin every time. The cache backend itself (blob storage,
// CDN, …) is configured server-side. Pass an empty patterns slice to
// disable caching for this session.
//
// @param blobName - server-side identifier of the snapshot to serve from
// @param patterns - URL wildcards to redirect to the cache; nil/empty disables
//
// @throws UNKNOWN_ERROR - the static paths could not be configured
//
// @example
//
//	_ = browser.SetStaticPaths(ctx, "snap-2026-05", []string{"*.example.com/*"})
func (c *CloudBrowser) SetStaticPaths(ctx context.Context, blobName string, patterns []string) error {
	_, err := c.client.SetStaticPaths(ctx, &generated.SetStaticPathsRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		BlobName: blobName,
		Patterns: patterns,
	})
	return err
}

// WaitForAnyRequest blocks until the next request whose URL matches one
// of the supplied patterns is observed.
//
// Returns the matched pattern's index and the captured request. When
// patterns[i].Abort is true the request is dropped with an empty 200
// response instead of being sent to the network.
//
// @param timeoutMs - per-call timeout in milliseconds; 0 uses the server default
// @param patterns - one or more URL patterns (with optional Abort flags)
//
// @returns int32 index of the matched pattern, *InterceptedRequest with
//
//	the captured method/URL/headers/body, and an error
//
// @throws UNKNOWN_ERROR - the wait timed out or no patterns were supplied
//
// @example
//
//	idx, req, err := browser.WaitForAnyRequest(ctx, 5000, []browserscale.RequestPattern{
//	    {URL: "*/api/login"},
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_ = idx
//	fmt.Println(req.Method, req.Url)
func (c *CloudBrowser) WaitForAnyRequest(ctx context.Context, timeoutMs float64, patterns []RequestPattern) (int32, *InterceptedRequest, error) {
	if len(patterns) == 0 {
		return -1, nil, errors.New("browserscale.WaitForAnyRequest: at least one pattern required")
	}
	urls, aborts := splitRequestPatterns(patterns)
	resp, err := c.client.WaitForAnyRequest(ctx, &generated.WaitForAnyRequestRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Patterns:   urls,
		AbortFlags: aborts,
		Timeout:    floatPtrIfNonZero(timeoutMs),
	})
	if err != nil {
		return -1, nil, err
	}
	return resp.Index, interceptedRequestFromProto(resp.Request), nil
}

// WaitForAnyResponse blocks until the next response whose URL matches one
// of the supplied patterns is observed.
//
// Same shape as [CloudBrowser.WaitForAnyRequest] but on the response phase.
// When patterns[i].Abort is true the page receives an empty 200 instead of
// the real response.
//
// @inheritDoc [CloudBrowser.WaitForAnyRequest]
//
// @returns int32 index of the matched pattern, *InterceptedResponse with
//
//	the captured status/headers/body, and an error
//
// @example
//
//	idx, resp, err := browser.WaitForAnyResponse(ctx, 5000, []browserscale.RequestPattern{
//	    {URL: "*/api/login"},
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_ = idx
//	fmt.Println(resp.StatusCode)
func (c *CloudBrowser) WaitForAnyResponse(ctx context.Context, timeoutMs float64, patterns []RequestPattern) (int32, *InterceptedResponse, error) {
	if len(patterns) == 0 {
		return -1, nil, errors.New("browserscale.WaitForAnyResponse: at least one pattern required")
	}
	urls, aborts := splitRequestPatterns(patterns)
	resp, err := c.client.WaitForAnyResponse(ctx, &generated.WaitForAnyResponseRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Patterns:   urls,
		AbortFlags: aborts,
		Timeout:    floatPtrIfNonZero(timeoutMs),
	})
	if err != nil {
		return -1, nil, err
	}
	return resp.Index, interceptedResponseFromProto(resp.Response), nil
}

// splitRequestPatterns turns a []RequestPattern into the proto's parallel
// URL + abort-flag slices. When no pattern has Abort set, the aborts
// slice is returned as nil so it stays off the wire.
func splitRequestPatterns(patterns []RequestPattern) (urls []string, aborts []int32) {
	urls = make([]string, len(patterns))
	any := false
	for i, p := range patterns {
		urls[i] = p.URL
		if p.Abort {
			any = true
		}
	}
	if !any {
		return urls, nil
	}
	aborts = make([]int32, len(patterns))
	for i, p := range patterns {
		if p.Abort {
			aborts[i] = 1
		}
	}
	return urls, aborts
}

// ModifyRequest waits for the next request whose URL matches urlPattern,
// applies the supplied header modifications (and optional body
// replacement), then forwards the modified request.
//
// One-shot: consumes the first matching request. Pass nil/empty mods to
// leave headers untouched and only override the body.
//
// @param urlPattern - URL wildcard to wait for
// @param body - replacement request body; empty leaves the original body
// @param timeoutMs - per-call timeout in milliseconds; 0 uses the server default
// @param mods - [HeaderModification] entries; see HeaderModification for the fields
//
// @returns *InterceptedRequest carrying the method/URL/headers/body that
//
//	were actually sent on the wire after modifications were applied
//
// @throws UNKNOWN_ERROR - no matching request appeared within the timeout
//
// @example
//
//	req, err := browser.ModifyRequest(ctx, "*/api/me", "", 5000, []browserscale.HeaderModification{
//	    {Action: browserscale.HeaderModificationAdd, Name: "X-Trace", Value: "abc123"},
//	    {Action: browserscale.HeaderModificationRemove, Name: "Cookie"},
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("forwarded headers:", req.Headers)
func (c *CloudBrowser) ModifyRequest(ctx context.Context, urlPattern, body string, timeoutMs float64, mods []HeaderModification) (*InterceptedRequest, error) {
	for _, m := range mods {
		switch m.Action {
		case HeaderModificationAdd, HeaderModificationEdit, HeaderModificationRemove:
		default:
			return nil, errors.New("browserscale.ModifyRequest: invalid HeaderModification.Action " + strconv.Quote(string(m.Action)))
		}
	}
	req := &generated.ModifyRequestRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		UrlPattern:    urlPattern,
		Modifications: headerModsToProto(mods),
		Timeout:       floatPtrIfNonZero(timeoutMs),
	}
	if body != "" {
		b := body
		req.Body = &b
	}
	resp, err := c.client.ModifyRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return interceptedRequestFromProto(resp.Request), nil
}

// ──────────────────────────────────────────────────────────────────────
// Network capture
// ──────────────────────────────────────────────────────────────────────

// NetworkBodies selects how much of a response body network capture keeps.
type NetworkBodies string

const (
	// NetworkBodiesNone keeps headers and status only. The default.
	NetworkBodiesNone NetworkBodies = "none"
	// NetworkBodiesText keeps bodies whose MIME type is textual — text/*,
	// JSON, XML, JavaScript, SVG.
	NetworkBodiesText NetworkBodies = "text"
	// NetworkBodiesAll keeps every body regardless of type. Binary payloads
	// (images, fonts, video) do not cross the browser boundary intact, so
	// prefer NetworkBodiesText unless you know the bodies are textual.
	NetworkBodiesAll NetworkBodies = "all"
)

// NetworkCaptureOptions configures [CloudBrowser.CaptureNetwork].
//
// There is deliberately no byte-cap option: buffer sizes bound memory on a
// machine shared with other sessions, so the server owns them.
type NetworkCaptureOptions struct {
	// Patterns are URL wildcards to capture; nil captures every request the
	// session makes. Prefix a pattern with "!" to exclude it, which is the
	// short way to say "everything except this".
	Patterns []string

	// Bodies selects response-body capture. Empty means NetworkBodiesNone.
	Bodies NetworkBodies

	// BodyPatterns narrows body capture to a subset of the captured requests;
	// nil applies Bodies to all of them. Use it to log every request but only
	// keep the payloads you care about.
	BodyPatterns []string
}

// NetworkExchangeHandler is called once per completed exchange.
//
// Calls are sequential and in the order the browser finished the requests, so
// the hops of a redirect chain arrive in order and the handler needs no locking
// of its own. It runs on a goroutine the SDK owns, not the caller's.
//
// Blocking here stalls the capture: the server buffers a bounded amount per
// reader and then drops its oldest entries, which [NetworkCapture.Dropped]
// reports. Hand slow work (disk, HTTP, a database) to another goroutine.
type NetworkExchangeHandler func(NetworkExchange)

// NetworkCapture is a running capture, returned by
// [CloudBrowser.CaptureNetwork]. Exchanges are delivered to the handler passed
// there; this handle only exists to stop the capture and to report how it went.
type NetworkCapture struct {
	browser *CloudBrowser
	stream  generated.Browser_StreamNetworkExchangesClient
	cancel  context.CancelFunc
	handler NetworkExchangeHandler

	// finished is closed once the reader goroutine has returned, meaning no
	// further handler call can be in flight.
	finished chan struct{}

	mu      sync.Mutex
	armed   bool
	err     error
	dropped uint64
	stopped bool
}

// CaptureNetwork starts capturing the session's network traffic and returns a
// live view of it.
//
// Every request matching opts.Patterns is reported once it completes, and
// "every request" is literal: capture sits in the browser process rather than
// in a page, so cross-process iframes, workers and service workers are
// included, the headers are the ones actually put on the wire (Cookie and
// Sec-* included), and each hop of a redirect chain arrives as its own
// exchange. Requests are never paused, so the page loads at full speed.
//
// This call returns as soon as the capture is running; onExchange then fires in
// the background while you drive the browser. The capture is armed only after
// the subscription exists, so nothing that happens after this call returns is
// missed. Call [NetworkCapture.Stop] when done — it disarms the capture
// server-side, which a cancelled context alone does not.
//
// @param opts - which requests to capture and whether to keep bodies
// @param onExchange - called per exchange; see [NetworkExchangeHandler] for the
//
//	ordering and blocking rules
//
// @returns *NetworkCapture handle for stopping the capture and inspecting how
//
//	it ended
//
// @throws UNKNOWN_ERROR - onExchange is nil, or the capture could not be started
//
// @example
//
//	capture, err := browser.CaptureNetwork(ctx, browserscale.NetworkCaptureOptions{
//	    Patterns: []string{"*/api/*"},
//	    Bodies:   browserscale.NetworkBodiesText,
//	}, func(ex browserscale.NetworkExchange) {
//	    fmt.Println(ex.StatusCode, ex.Method, ex.Url)
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer capture.Stop(ctx)
//
//	_, _ = browser.Navigate(ctx, "https://example.com", 0)
func (c *CloudBrowser) CaptureNetwork(ctx context.Context, opts NetworkCaptureOptions, onExchange NetworkExchangeHandler) (*NetworkCapture, error) {
	nc, err := c.StreamNetworkExchanges(ctx, onExchange)
	if err != nil {
		return nil, err
	}
	if err := c.StartNetworkCapture(ctx, opts); err != nil {
		nc.cancel()
		return nil, err
	}
	nc.mu.Lock()
	nc.armed = true
	nc.mu.Unlock()
	return nc, nil
}

// StartNetworkCapture arms a capture without subscribing to it.
//
// Use it when the reader lives somewhere else — another process, or a later
// [ConnectSession] against the same session. Most callers want
// [CloudBrowser.CaptureNetwork] instead, which arms and subscribes together.
// Calling this again replaces the running capture.
//
// @param opts - which requests to capture and whether to keep bodies
//
// @throws UNKNOWN_ERROR - the capture could not be started
func (c *CloudBrowser) StartNetworkCapture(ctx context.Context, opts NetworkCaptureOptions) error {
	_, err := c.client.StartNetworkCapture(ctx, &generated.StartNetworkCaptureRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Patterns:     opts.Patterns,
		Bodies:       string(opts.Bodies),
		BodyPatterns: opts.BodyPatterns,
	})
	return err
}

// StopNetworkCapture disarms the session's capture.
//
// @returns bool reporting whether a capture was running, and an error
//
// @throws UNKNOWN_ERROR - the capture could not be stopped
func (c *CloudBrowser) StopNetworkCapture(ctx context.Context) (bool, error) {
	resp, err := c.client.StopNetworkCapture(ctx, &generated.StopNetworkCaptureRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return false, err
	}
	return resp.Stopped, nil
}

// StreamNetworkExchanges subscribes to the session's capture without arming
// one, for reading a capture that [CloudBrowser.StartNetworkCapture] armed
// elsewhere. Several readers can watch the same capture, each with its own
// buffer.
//
// Stopping the returned view detaches this reader and leaves the capture
// running, since other readers may still be attached.
//
// @param onExchange - called per exchange; see [NetworkExchangeHandler] for the
//
//	ordering and blocking rules
//
// @returns *NetworkCapture attached to whatever capture is running; onExchange
//
//	simply never fires when none is
//
// @throws UNKNOWN_ERROR - onExchange is nil, or the subscription could not be opened
func (c *CloudBrowser) StreamNetworkExchanges(ctx context.Context, onExchange NetworkExchangeHandler) (*NetworkCapture, error) {
	if onExchange == nil {
		return nil, errors.New("browserscale.StreamNetworkExchanges: onExchange must not be nil")
	}
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := c.client.StreamNetworkExchanges(streamCtx, &generated.StreamNetworkExchangesRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	// Opening a stream does not wait for the server to start handling it, so
	// arming a capture straight after could outrun the subscription and lose
	// the first exchanges. The server sends its headers once subscribed;
	// blocking for them here makes the order deterministic.
	if _, err := stream.Header(); err != nil {
		cancel()
		return nil, err
	}
	nc := &NetworkCapture{
		browser:  c,
		stream:   stream,
		cancel:   cancel,
		handler:  onExchange,
		finished: make(chan struct{}),
	}
	go nc.pump()
	return nc, nil
}

// Wait blocks until the capture ends — [NetworkCapture.Stop], a cancelled
// context, a dead session or a transport failure — and returns
// [NetworkCapture.Err].
//
// Use it to capture for as long as the session lives. It is not needed when you
// drive the browser yourself and call Stop when done.
func (nc *NetworkCapture) Wait() error {
	<-nc.finished
	return nc.Err()
}

// Err reports why the capture ended. It returns nil while the capture is still
// running, and after a clean stop, a cancelled context, or the session ending
// normally.
func (nc *NetworkCapture) Err() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.err
}

// Dropped reports how many exchanges the server discarded because this reader
// fell behind. Anything above zero means the log has holes: make the handler
// cheaper, narrow Patterns, or stop capturing bodies.
func (nc *NetworkCapture) Dropped() uint64 {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.dropped
}

// Stop ends the capture. Idempotent, and safe to defer.
//
// Once it returns, the handler is no longer running and everything it wrote is
// visible to the calling goroutine — so a handler may append to a slice without
// locking, as long as you only read that slice after Stop. Views from
// [CloudBrowser.StreamNetworkExchanges] only detach — they never disarm a
// capture other readers may share.
//
// To stop from inside the handler, call [CloudBrowser.StopNetworkCapture]
// instead: Stop waits for the handler to return, so calling it from there would
// wait on itself until ctx expires.
//
// ctx covers the disarm call, so pass a live one: the context the capture was
// created with may already be cancelled by the time you stop.
//
// @throws UNKNOWN_ERROR - the capture could not be disarmed; the local reader is
//
//	shut down regardless
func (nc *NetworkCapture) Stop(ctx context.Context) error {
	nc.mu.Lock()
	if nc.stopped {
		nc.mu.Unlock()
		return nil
	}
	nc.stopped = true
	armed := nc.armed
	nc.mu.Unlock()

	var err error
	if armed {
		_, err = nc.browser.StopNetworkCapture(ctx)
	}
	nc.cancel()
	select {
	case <-nc.finished:
	case <-ctx.Done():
	}
	return err
}

// pump reads the gRPC stream and calls the handler for each exchange. Running
// the handler inline is what makes delivery ordered and single-threaded; a slow
// handler applies backpressure to the server's buffer, which drops its oldest
// entries rather than stalling the browser.
func (nc *NetworkCapture) pump() {
	defer close(nc.finished)
	for {
		event, err := nc.stream.Recv()
		if err != nil {
			if !isStreamEnd(err) {
				nc.mu.Lock()
				nc.err = err
				nc.mu.Unlock()
			}
			return
		}
		if event.Dropped > 0 {
			nc.mu.Lock()
			nc.dropped = event.Dropped
			nc.mu.Unlock()
		}
		if event.Exchange == nil {
			continue
		}
		nc.handler(networkExchangeFromProto(event.Exchange))
	}
}

// isStreamEnd separates an ordinary end of stream — the server closed it, or we
// cancelled it ourselves in Stop — from a real transport failure worth
// surfacing through Err.
func isStreamEnd(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return true
	}
	switch status.Code(err) {
	case codes.OK, codes.Canceled:
		return true
	}
	return false
}
