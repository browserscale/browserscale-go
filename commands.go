package browserscale

import (
	"context"
	"encoding/json"

	"github.com/browserscale/browserscale-go/generated"
)

// ── Context-level ──

// SetProxy changes the runtime proxy for this session.
//
// Takes effect for new requests immediately; in-flight requests keep their
// original routing. Pass an empty proxyHost to clear the proxy and route
// directly.
//
// @param proxyHost - upstream proxy host; empty disables the proxy
// @param proxyPort - upstream proxy port; ignored when proxyHost is empty
// @param proxyUsername - proxy auth user (empty for unauthenticated proxies)
// @param proxyPassword - proxy auth password (empty for unauthenticated proxies)
//
// @throws UNKNOWN_ERROR - the proxy could not be applied
//
// @example
//
//	_ = browser.SetProxy(ctx, "proxy.example.com", 8080, "user", "pass")
func (c *CloudBrowser) SetProxy(ctx context.Context, proxyHost string, proxyPort int32, proxyUsername, proxyPassword string) error {
	req := &generated.SetProxyRequest{SessionId: c.sessionId, ApiKey: c.apiKey}
	if proxyHost != "" {
		req.ProxyHost = &proxyHost
		p := proxyPort
		req.ProxyPort = &p
		if proxyUsername != "" {
			u := proxyUsername
			req.ProxyUsername = &u
		}
		if proxyPassword != "" {
			pw := proxyPassword
			req.ProxyPassword = &pw
		}
	}
	_, err := c.client.SetProxy(ctx, req)
	return err
}

// GetPages returns all open pages (tabs and popups) for this session's
// browser context.
//
// Each [PageInfo] carries the page's URL, title, viewport and a full nested
// frame tree (out-of-process iframes are children of the page's main frame).
//
// @returns []*PageInfo for every page currently open in the context
//
// @throws UNKNOWN_ERROR - the pages could not be enumerated
//
// @example
//
//	pages, err := browser.GetPages(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, p := range pages {
//	    fmt.Println(p.Url, p.Title)
//	}
func (c *CloudBrowser) GetPages(ctx context.Context) ([]*PageInfo, error) {
	resp, err := c.client.GetPages(ctx, &generated.GetPagesRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*PageInfo, len(resp.Pages))
	for i, p := range resp.Pages {
		out[i] = pageInfoFromProto(p)
	}
	return out, nil
}

// ── Page navigation / content ──

// Navigate navigates the page to url.
//
// Returns once the primary main-frame navigation commits (the response is
// received and a new document is selected), before DOMContentLoaded or load
// fire. Cross-origin redirects are followed.
//
// @param url - destination URL
// @param timeoutMs - per-call timeout in milliseconds; 0 uses the server default
//
// @returns *NavigateResult with the final resolved URL and the frameId of
//
//	the main frame after navigation
//
// @throws UNKNOWN_ERROR - the navigation failed or timed out
//
// @example
//
//	_, err := browser.Navigate(ctx, "https://example.com", 0)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) Navigate(ctx context.Context, url string, timeoutMs float64) (*NavigateResult, error) {
	resp, err := c.client.Navigate(ctx, &generated.NavigateRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Url:     url,
		Timeout: floatPtrIfNonZero(timeoutMs),
	})
	if err != nil {
		return nil, err
	}
	return &NavigateResult{FrameId: resp.FrameId, Url: resp.Url}, nil
}

// LoadHTML serves a synthetic response for the next navigation to url.
//
// Registers a one-shot interceptor that intercepts the next request to url
// and replies with the supplied html and headers instead of going to the
// network. Useful for snapshotted pages, test fixtures, and offline replays.
// Pair with [CloudBrowser.Navigate] to trigger the load.
//
// @param url - the URL pattern that, when navigated to, returns the html
// @param html - the response body to serve
// @param headers - extra response headers (Content-Type is set automatically)
// @param statusCode - HTTP status code to serve; 0 means 200
//
// @throws UNKNOWN_ERROR - the interceptor could not be installed
//
// @example
//
//	_ = browser.LoadHTML(ctx, "https://example.com", "<h1>hi</h1>", nil, 0)
//	_, _ = browser.Navigate(ctx, "https://example.com", 0)
func (c *CloudBrowser) LoadHTML(ctx context.Context, url, html string, headers []Header, statusCode int32) error {
	req := &generated.LoadHTMLRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Url:     url,
		Html:    html,
		Headers: headersToProto(headers),
	}
	if statusCode != 0 {
		s := statusCode
		req.StatusCode = &s
	}
	_, err := c.client.LoadHTML(ctx, req)
	return err
}

// ── Evaluation ──

// Evaluate runs a JavaScript expression in the page's main frame.
//
// The expression's return value is JSON-serialized server-side and parsed
// eagerly into [EvaluateResult.Value]. When the expression returns a DOM
// element the [EvaluateResult.Value] is left empty and the element metadata
// (BackendNodeId, IsVisible, Bounds) is populated instead — use Node(id)
// in subsequent calls to act on it.
//
// @param expression - JavaScript expression evaluated in the main frame
//
// @returns *EvaluateResult with either Value (for non-Element returns) or
//
//	BackendNodeId + IsVisible + Bounds (for Element returns)
//
// @throws UNKNOWN_ERROR - the expression threw or could not be compiled
//
// @example
//
//	res, err := browser.Evaluate(ctx, "document.title")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(res.Value)
func (c *CloudBrowser) Evaluate(ctx context.Context, expression string) (*EvaluateResult, error) {
	return c.evaluate(ctx, "", expression)
}

// EvaluateInFrame runs a JavaScript expression in the given frame.
//
// Same semantics as [CloudBrowser.Evaluate] but targets a specific frame
// instead of the main frame. Useful for evaluating inside OOPIFs (out-of-
// process iframes) found via [CloudBrowser.GetPages].
//
// @inheritDoc [CloudBrowser.Evaluate]
// @param frameId - id of the frame to evaluate in; empty falls back to the main frame
//
// @example
//
//	pages, _ := browser.GetPages(ctx)
//	iframeId := pages[0].FrameTree.Children[0].FrameId
//	_, _ = browser.EvaluateInFrame(ctx, iframeId, "location.href")
func (c *CloudBrowser) EvaluateInFrame(ctx context.Context, frameId, expression string) (*EvaluateResult, error) {
	return c.evaluate(ctx, frameId, expression)
}

func (c *CloudBrowser) evaluate(ctx context.Context, frameId, expression string) (*EvaluateResult, error) {
	resp, err := c.client.Evaluate(ctx, &generated.EvaluateRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Expression: expression,
		FrameId:    strPtr(frameId),
	})
	if err != nil {
		return nil, err
	}
	out := &EvaluateResult{
		BackendNodeId: resp.BackendNodeId,
		IsVisible:     resp.IsVisible,
		Bounds:        rectFromProto(resp.Bounds),
	}
	if resp.Result != "" {
		if err := json.Unmarshal([]byte(resp.Result), &out.Value); err != nil {
			out.Value = resp.Result
		}
	}
	return out, nil
}

// ── Waiting ──
//
// See wait.go for the Wait(ctx, args...) entry point that uses the variadic
// functional-options API. The underlying RPC is WaitForAny.

// ── Element actions ──
//
// Implemented in their own files using the variadic functional-options API:
//   Click        → click.go
//   MoveTo       → move.go
//   ScrollTo     → scroll.go
//   Drag         → drag.go
//   SelectOption → select.go
//   Fill         → fill.go

// ── Network interception ──
//
// Implemented in network.go:
//   SetBlockList, SetStaticPaths,
//   WaitForAnyRequest, WaitForAnyResponse, ModifyRequest.

// ── Cookies ──
//
// Implemented in cookies.go: GetCookies, SetCookies, ClearCookies, and
// CookieParam structs for browser cookie attributes.

// ── DOM / observation ──

// GetDOM returns a JSON string in CDP DOM.Node shape for the requested frame.
//
// The shape matches Chrome DevTools' Protocol DOM.Node — useful for piping
// into agent loops or visualizers that already speak CDP. For a much smaller
// agent-oriented payload, prefer [CloudBrowser.GetObservation] instead.
//
// @param frameId - id of the frame to dump; empty targets the main frame
// @param depth - tree depth: -1 for the full tree, 0 for root only, N for
//
//	root + N descendant levels
//
// @returns JSON string in CDP DOM.Node shape
//
// @throws UNKNOWN_ERROR - the DOM could not be retrieved
//
// @example
//
//	tree, err := browser.GetDOM(ctx, "", -1)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(tree)
func (c *CloudBrowser) GetDOM(ctx context.Context, frameId string, depth int32) (string, error) {
	resp, err := c.client.GetDOM(ctx, &generated.GetDOMRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		FrameId: strPtr(frameId),
		Depth:   intPtr(depth),
	})
	if err != nil {
		return "", err
	}
	return resp.Dom, nil
}

// GetObservation returns a compact, frame-aware view of the visible page —
// the first thing to reach for on an unfamiliar page, and the cheapest way to
// re-read the current state afterwards.
//
// Each frame opens with header lines carrying the URL, the title and the
// scroll position, then one line per visible element:
//
//	input#email[47] type="email" name="loginId" value="a@b.com" required click "E-Mail"
//
// It spans every frame, pierces open and closed shadow roots, enumerates
// <select> options, and reports live form state: value= is what is typed in
// right now (passwords as a length), checked= for boxes. The trailing quoted
// string is always the label or text, never the value, so an empty and a
// prefilled field stay distinguishable. Because the headers already carry URL,
// title and scroll offset, this replaces the usual handful of [CloudBrowser.Evaluate]
// probes after each step.
//
// On what to do with the result: backendNodeId (the 47 above) is a handle for
// this session and can be passed straight to click/fill via [Node]. It does not
// survive a new document, so for anything you write into a script, target with
// [CSS] or [JS] instead — those calls return the backendNodeId they resolved to,
// which lets you confirm the durable anchor hits the element you saw.
//
// @returns the observation in the requested format, ready to hand to a model
//
// @throws UNKNOWN_ERROR - the observation could not be produced
//
// @example
//
//	obs, err := browser.GetObservation(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(obs)
func (c *CloudBrowser) GetObservation(ctx context.Context) (string, error) {
	return c.getObservation(ctx, ObservationOpts{})
}

// GetObservationWith is the customizable variant of [CloudBrowser.GetObservation].
//
// @inheritDoc [CloudBrowser.GetObservation]
// @param opts - observation customization; see [ObservationOpts]
//
// @example
//
//	// Only what is on screen right now, as structured JSON.
//	obs, err := browser.GetObservationWith(ctx, browserscale.ObservationOpts{
//	    Format:       "json",
//	    ViewportOnly: true,
//	})
//
//	// Re-read just one form after the first full look.
//	obs, err = browser.GetObservationWith(ctx, browserscale.ObservationOpts{
//	    Selector: "form#register",
//	})
func (c *CloudBrowser) GetObservationWith(ctx context.Context, opts ObservationOpts) (string, error) {
	return c.getObservation(ctx, opts)
}

func (c *CloudBrowser) getObservation(ctx context.Context, o ObservationOpts) (string, error) {
	req := &generated.GetObservationRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		MaxElementsPerFrame: intPtr(o.MaxElementsPerFrame),
		MaxTextLength:       intPtr(o.MaxTextLength),
		MaxTotalTokens:      intPtr(o.MaxTotalTokens),
		BackendNodeId:       intPtr(o.BackendNodeId),
		Selector:            strPtr(o.Selector),
		JsExpression:        strPtr(o.JSExpression),
		FrameId:             strPtr(o.InFrame),
	}
	if o.Format != "" {
		req.Format = strPtr(o.Format)
	}
	if o.IncludeBounds {
		req.IncludeBounds = Ptr(true)
	}
	if o.ViewportOnly {
		req.ViewportOnly = Ptr(true)
	}
	resp, err := c.client.GetObservation(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Observation, nil
}

// ── Screenshot ──

// Screenshot captures a single image of the page's current frame and returns
// it as base64-encoded image bytes.
//
// The capture uses a one-shot surface copy (the same mechanism as CDP
// Page.captureScreenshot), so it is independent of any active live stream and
// works with both GPU (hardware) and software compositing.
//
// @param format - "png" (default), "jpeg", or "webp"; pass "" for PNG
//
// @param quality - encode quality 0-100 for "jpeg"/"webp" (ignored for
//
//	"png"); pass 0 to use the server default (90)
//
// @returns *ScreenshotResult with the base64 image in DataBase64 and the
//
//	physical pixel Width/Height
//
// @throws UNKNOWN_ERROR - the screenshot could not be captured
//
// @example
//
//	shot, err := browser.Screenshot(ctx, "png", 0)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	img, _ := base64.StdEncoding.DecodeString(shot.DataBase64)
//	os.WriteFile("page.png", img, 0o644)
func (c *CloudBrowser) Screenshot(ctx context.Context, format string, quality int32) (*ScreenshotResult, error) {
	resp, err := c.client.Screenshot(ctx, &generated.ScreenshotRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Format:  strPtr(format),
		Quality: intPtr(quality),
	})
	if err != nil {
		return nil, err
	}
	return &ScreenshotResult{
		DataBase64: resp.DataBase64,
		Width:      resp.Width,
		Height:     resp.Height,
	}, nil
}
