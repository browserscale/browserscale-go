package browserscale

import (
	"context"
	"errors"
	"strconv"

	"github.com/browserscale/browserscale-go/generated"
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
