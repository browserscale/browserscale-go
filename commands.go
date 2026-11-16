package browserscale

import (
	"context"
	"encoding/json"
	"errors"

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
// @param opts - [ProxyWithScheme] and [ProxyWithMasque], as for a rental
//
// Unlike a rental, the new proxy is not checked first. An invalid scheme or
// MASQUE address is refused with INVALID_PROXY; otherwise this reports only
// transport failures - a dead session, a broken connection.
//
// @example
//
//	_ = browser.SetProxy(ctx, "proxy.example.com", 8080, "user", "pass")
//	_ = browser.SetProxy(ctx, "proxy.example.com", 1080, "user", "pass",
//	    browserscale.ProxyWithScheme(browserscale.ProxySOCKS5))
func (c *CloudBrowser) SetProxy(ctx context.Context, proxyHost string, proxyPort int32, proxyUsername, proxyPassword string, opts ...ProxyOption) error {
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
		var o proxyOptions
		for _, opt := range opts {
			opt(&o)
		}
		if o.scheme != "" {
			s := string(o.scheme)
			req.ProxyScheme = &s
		}
		if o.masque != "" {
			m := o.masque
			req.MasqueProxy = &m
		}
	}
	resp, err := c.client.SetProxy(ctx, req)
	if err != nil {
		return err
	}
	return commandErrorFrom("setProxy", resp.GetError())
}

// ProxyOption sets an optional part of the proxy passed to
// [CloudBrowser.SetProxy].
type ProxyOption func(*proxyOptions)

type proxyOptions struct {
	scheme ProxyScheme
	masque string
}

// ProxyWithScheme sets the proxy's protocol, as
// [BrowserConfig.WithProxyScheme] does for a rental.
func ProxyWithScheme(scheme ProxyScheme) ProxyOption {
	return func(o *proxyOptions) { o.scheme = scheme }
}

// ProxyWithMasque carries HTTP/3 through a MASQUE proxy ("host:port") with
// the proxy's credentials, as [BrowserConfig.WithMasque] does for a rental.
func ProxyWithMasque(hostPort string) ProxyOption {
	return func(o *proxyOptions) { o.masque = hostPort }
}

// Pages returns all open pages (tabs and popups) for this session's
// browser context.
//
// Each [PageInfo] carries the [Page] to act on, its URL, title, viewport, a
// full nested frame tree (out-of-process iframes are children of the page's
// main frame), whether it has the focus, and where it came from.
//
// @returns []*PageInfo for every page currently open in the context
//
// Reports only transport failures - a dead session, a broken connection. This
// call has no semantic failure of its own, so there are no error codes to
// branch on.
//
// @example
//
//	pages, err := browser.Pages(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, p := range pages {
//	    if p.IsPopup && p.Opener != nil {
//	        fmt.Println("popup", p.Url, "opened by", p.Opener.Id())
//	    }
//	}
func (c *CloudBrowser) Pages(ctx context.Context) ([]*PageInfo, error) {
	resp, err := c.client.GetPages(ctx, &generated.GetPagesRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("getPages", resp.GetError()); e != nil {
		return nil, e
	}
	out := make([]*PageInfo, len(resp.Pages))
	for i, p := range resp.Pages {
		out[i] = c.pageInfoFromProto(p)
	}
	return out, nil
}

// GetUsage reports what this session's browser has consumed so far.
//
// Everything but MinMemory and AverageMemory only ever grows, so polling and
// diffing two readings gives the cost of what ran in between. The final figures
// need no call of their own: [CloudBrowser.StopBrowser] returns them.
//
// @returns *SessionUsage as of now
//
// Reports only transport failures - a dead session, a broken connection. This
// call has no semantic failure of its own, so there are no error codes to
// branch on.
//
// @example
//
//	before, _ := browser.GetUsage(ctx)
//	_, _ = page.Navigate(ctx, "https://example.com", 0)
//	after, _ := browser.GetUsage(ctx)
//	fmt.Printf("navigation cost %.2fs of CPU\n", after.CpuTime-before.CpuTime)
func (c *CloudBrowser) GetUsage(ctx context.Context) (*SessionUsage, error) {
	resp, err := c.client.GetUsage(ctx, &generated.GetUsageRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("getUsage", resp.GetError()); e != nil {
		return nil, e
	}
	if resp.Usage == nil {
		return nil, errors.New("getUsage: the server sent no usage")
	}
	return sessionUsageFromProto(resp.Usage), nil
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
// @throws timeout - nothing committed before the deadline; the page may still be
// loading, so a longer timeout can be the whole fix
// @throws net_error - the URL never loaded: DNS, TLS, a refused connection, or a
// proxy that could not reach it. The message carries the underlying net error
// name, which is what separates a bad proxy from a bad host - worth logging
// @throws crashed - the renderer died mid-navigation; the page is unusable and
// has to be navigated again
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	_, err := page.Navigate(ctx, "https://example.com", 0)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (p *Page) Navigate(ctx context.Context, url string, timeoutMs float64) (*NavigateResult, error) {
	resp, err := p.client.Navigate(ctx, &generated.NavigateRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		Url:     url,
		Timeout: floatPtrIfNonZero(timeoutMs),
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("navigate", resp.GetError()); e != nil {
		return nil, e
	}
	return &NavigateResult{FrameId: resp.FrameId, Url: resp.Url}, nil
}

// LoadHTML navigates the page to url and serves html as the response.
//
// The page navigates as with [Page.Navigate], but the request for url never
// reaches the network: it is answered with the supplied html, headers and
// status code. The answer is used once; a later navigation to url, also a
// reload, goes to the network again. Useful for snapshotted pages, test
// fixtures, and offline replays. It returns once the response was handed to
// the page, so wait for what the next step needs as after a navigation.
//
// @param url - the URL the page will show the html under
// @param html - the response body to serve
// @param headers - extra response headers (Content-Type is set automatically)
// @param statusCode - HTTP status code to serve; 0 means 200
//
// @throws timeout - the page never requested the URL, so the prepared response
// had nobody to hand it to; usually the navigation was cancelled or redirected
// away before reaching it
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	_ = page.LoadHTML(ctx, "https://example.com", "<h1>hi</h1>", nil, 0)
//	_, _ = page.Wait(ctx, browserscale.CSS("h1"))
func (p *Page) LoadHTML(ctx context.Context, url, html string, headers []Header, statusCode int32) error {
	req := &generated.LoadHTMLRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		Url:     url,
		Html:    html,
		Headers: headersToProto(headers),
	}
	if statusCode != 0 {
		s := statusCode
		req.StatusCode = &s
	}
	resp, err := p.client.LoadHTML(ctx, req)
	if err != nil {
		return err
	}
	return commandErrorFrom("loadHTML", resp.GetError())
}

// ── Evaluation ──

// Evaluate runs a JavaScript expression in the page's main frame.
//
// The expression's return value is JSON-serialized server-side and parsed
// eagerly into [EvaluateResult.Value]. When the expression returns a DOM
// element the [EvaluateResult.Value] is left empty and the element metadata
// (Ref, IsVisible, Bounds) is populated instead — pass the Ref to
// subsequent calls to act on it.
//
// A falsy answer and a broken expression are different outcomes. Returning
// null, false or undefined is a successful evaluation and comes back as a
// result; an expression that throws or will not compile comes back as a
// [*CommandError], so a typo can never read as "the page says null".
//
// @param expression - JavaScript expression evaluated in the main frame
//
// @returns *EvaluateResult with either Value (for non-Element returns) or
//
//	Ref + IsVisible + Bounds (for Element returns)
//
// @throws threw - the expression raised; the message carries the exception text
// @throws not_run - it could not be compiled, or execution never started
// @throws aborted - execution was stopped by the browser before it finished
// @throws no_context - the frame had no live script context to evaluate in
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	res, err := page.Evaluate(ctx, "document.title")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(res.Value)
//
// @example
//
//	// Telling a false answer from a broken expression.
//	res, err := page.Evaluate(ctx, "window.__ready === true")
//	var ce *browserscale.CommandError
//	if errors.As(err, &ce) {
//	    log.Fatalf("expression is broken: %v", ce)
//	}
//	if res.Value != true {
//	    // legitimately not ready yet
//	}
func (p *Page) Evaluate(ctx context.Context, expression string) (*EvaluateResult, error) {
	return p.evaluate(ctx, nil, expression)
}

// EvaluateIn runs a JavaScript expression inside a container: in the
// document of a frame, or scoped to an element.
//
// Same semantics as [Page.Evaluate], but the expression runs where the
// container leads:
//
//   - a frame ([FrameURL], [FrameName], [FrameByID], or a [CSS], [JS] or
//     reference locator naming an iframe element): that frame's document,
//     same-origin or not
//   - any other element: the document the element lives in, with the element
//     itself available to the expression as __wrc.scope
//
// The container is looked up once, as the page is at that moment; it does
// not wait for a frame that is still to come. Use [Page.Wait] first when it
// may not be there yet. The result's Frame says which frame the expression
// ran in.
//
// @inheritDoc [Page.Evaluate]
// @param container - where the expression runs; see [Locator.In]
//
// @throws container_not_found - the container, or one it is itself put in,
// is not on the page; the message names the missing link
// @throws outside_scope - the expression returned an element outside the
// element the container confined it to
// @throws [StaleRefError] - the container is a reference whose element is gone
//
// @example
//
//	pay := browserscale.CSS("iframe[title='Secure payment']")
//	title, err := page.EvaluateIn(ctx, pay, "document.title")
//
// @example
//
//	// Scoped to an element: how many rows does this table have?
//	table := browserscale.CSS("table#orders")
//	rows, err := page.EvaluateIn(ctx, table, "__wrc.scope.rows.length")
func (p *Page) EvaluateIn(ctx context.Context, container *Locator, expression string) (*EvaluateResult, error) {
	if err := container.validateContainer("EvaluateIn"); err != nil {
		return nil, err
	}
	if err := container.validateChain("EvaluateIn"); err != nil {
		return nil, err
	}
	if container.anyFrame {
		return nil, errors.New("browserscale.EvaluateIn: AnyFrame names every frame, and an expression runs in one; name the frame")
	}
	return p.evaluate(ctx, container, expression)
}

func (p *Page) evaluate(ctx context.Context, container *Locator, expression string) (*EvaluateResult, error) {
	resp, err := p.client.Evaluate(ctx, &generated.EvaluateRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		Expression: expression,
		In:         container.chainWithSelf(),
	})
	if err != nil {
		return nil, err
	}
	out := &EvaluateResult{
		Success:   resp.Success,
		Ref:       refLocator(resp.Ref, resp.Frame),
		Frame:     frameRefFromProto(resp.Frame),
		IsVisible: resp.IsVisible,
		Bounds:    rectFromProto(resp.Bounds),
	}
	if resp.Result != "" {
		if err := json.Unmarshal([]byte(resp.Result), &out.Value); err != nil {
			out.Value = resp.Result
		}
	}
	// An expression that threw arrives as success=false in the payload, not as a
	// transport error. Surfaced through err so the res, err shape is unchanged.
	if e := resp.GetError(); e != nil {
		return out, p.checked(ctx, container, &CommandError{Command: "evaluate", Code: e.Code, Message: e.Message})
	}
	return out, nil
}

// Frames returns the page's frame tree: the main frame, and below it every
// iframe, nested the way the documents are.
//
// Each [FrameInfo] says what a frame shows (URL, name), where it sits and
// whether a person can see it. It is for looking around: to find out which
// frames a page has, and what would tell one from the others in a [FrameURL]
// or [FrameName]. To act in a frame straight away, [FrameInfo.Locator] hands
// it back as a container.
//
// A frame is not an element, so the tree carries no reference to the iframe
// elements. Where the iframe element itself is needed (to click or scroll
// it), name it with [CSS] or take it from [Page.Wait].
//
// @returns *FrameInfo of the main frame, with its Children
//
// @throws [PageClosedError] - the page has closed
//
// @example
//
//	frames, err := page.Frames(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	frames.Walk(func(f *browserscale.FrameInfo, depth int) bool {
//	    fmt.Printf("%s%s name=%q visible=%v\n", strings.Repeat("  ", depth), f.Url, f.Name, f.IsVisible)
//	    return true
//	})
func (p *Page) Frames(ctx context.Context) (*FrameInfo, error) {
	resp, err := p.client.GetPages(ctx, &generated.GetPagesRequest{
		SessionId: p.sessionId, ApiKey: p.apiKey,
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("getPages", resp.GetError()); e != nil {
		return nil, e
	}
	for _, pg := range resp.GetPages() {
		if pg.GetPageId() == p.id {
			if f := frameInfoFromProto(pg.GetFrameTree()); f != nil {
				return f, nil
			}
			return &FrameInfo{Url: pg.GetUrl()}, nil
		}
	}
	// The page is not among the open ones: it has closed. Say so the way any
	// other call on it does, with the page that has the focus now.
	focused, err := p.browser.FocusedPage(ctx)
	if err != nil {
		return nil, err
	}
	return nil, &PageClosedError{PageId: p.id, FocusedPage: focused}
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

// ── Auth / DBSC ──
//
// Implemented in auth_session.go: GetAuthSession, SetAuthSession, and
// AuthSession / DbscSession for portable signed-in personas.

// ── DOM / observation ──

// GetDOM returns a JSON string in CDP DOM.Node shape for the requested frame.
//
// The shape matches Chrome DevTools' Protocol DOM.Node — useful for piping
// into agent loops or visualizers that already speak CDP. For a much smaller
// agent-oriented payload, prefer [Page.GetObservation] instead.
//
// @param frameId - id of the frame to dump; empty targets the main frame
// @param depth - tree depth: -1 for the full tree, 0 for root only, N for
//
//	root + N descendant levels
//
// @returns JSON string in CDP DOM.Node shape
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	tree, err := page.GetDOM(ctx, "", -1)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(tree)
func (p *Page) GetDOM(ctx context.Context, frameId string, depth int32) (string, error) {
	resp, err := p.client.GetDOM(ctx, &generated.GetDOMRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		FrameId: strPtr(frameId),
		Depth:   intPtr(depth),
	})
	if err != nil {
		return "", err
	}
	if e := commandErrorFrom("getDOM", resp.GetError()); e != nil {
		return "", e
	}
	return resp.Dom, nil
}

// GetObservation returns a compact, frame-aware view of the visible page —
// the first thing to reach for on an unfamiliar page, and the cheapest way to
// re-read the current state afterwards.
//
// Each frame opens with header lines carrying the URL, the title and the
// scroll position, then one line per element a person can see or act on:
//
//	input#email[47] type="email" name="loginId" value="a@b.com" required click "E-Mail"
//
// It spans every frame, pierces open and closed shadow roots, enumerates
// <select> options, and reports live state: value= is what is typed in right
// now (passwords as a length), checked= for boxes, expanded= for menus and
// accordions, and flags such as selected, focused, disabled or invalid. The
// trailing quoted string is always the label or text, never the value, so an
// empty and a prefilled field stay distinguishable. An element that cannot be
// seen is listed only when it is interactive, with the reason as hidden="...".
// When a dialog or a consent layer lies over the page, a header line
// "# on top: ..." names it, and its rows carry the flag on-top. Because the
// headers already carry URL, title and scroll offset, this replaces the usual
// handful of [Page.Evaluate] probes after each step.
//
// On what to do with the result: each frame's header carries a line
// "# ref @<document>", and a row's number (the 47 above) followed by that
// suffix is the element's reference: pass it to click/fill via [Ref]. It is a
// handle for this session and does not survive a new document, so for anything
// you write into a script, target with [CSS] or [JS] instead — those calls
// return the reference they resolved to, which lets you confirm the durable
// anchor hits the element you saw.
//
// @returns the observation in the requested format, ready to hand to a model
//
// @throws not_found - the requested scope root is not on the page, so there was
// nothing to observe. Distinct from an observation that comes back empty, which
// means the scope exists and holds nothing worth reporting
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	obs, err := page.GetObservation(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(obs)
func (p *Page) GetObservation(ctx context.Context) (string, error) {
	return p.getObservation(ctx, ObservationOpts{})
}

// GetObservationWith is the customizable variant of [Page.GetObservation].
//
// @inheritDoc [Page.GetObservation]
// @param opts - observation customization; see [ObservationOpts]
//
// @example
//
//	// Only what is on screen right now, as structured JSON.
//	obs, err := page.GetObservationWith(ctx, browserscale.ObservationOpts{
//	    Format:       "json",
//	    ViewportOnly: true,
//	})
//
//	// Re-read just one form after the first full look.
//	obs, err = page.GetObservationWith(ctx, browserscale.ObservationOpts{
//	    Scope: browserscale.CSS("form#register"),
//	})
//
//	// Only what one frame shows.
//	obs, err = page.GetObservationWith(ctx, browserscale.ObservationOpts{
//	    Scope: browserscale.FrameURL("*checkout.example.com/*"),
//	})
func (p *Page) GetObservationWith(ctx context.Context, opts ObservationOpts) (string, error) {
	return p.getObservation(ctx, opts)
}

func (p *Page) getObservation(ctx context.Context, o ObservationOpts) (string, error) {
	req := &generated.GetObservationRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		MaxElementsPerFrame: intPtr(o.MaxElementsPerFrame),
		MaxTextLength:       intPtr(o.MaxTextLength),
		MaxTotalTokens:      intPtr(o.MaxTotalTokens),
	}
	if s := o.Scope; s != nil {
		if err := s.validateContainer("GetObservation"); err != nil {
			return "", err
		}
		if err := s.validateChain("GetObservation"); err != nil {
			return "", err
		}
		if s.isElement() && !s.mustBeFrame {
			// An element: its subtree, found where the locator says.
			req.Ref, req.Selector, req.JsExpression = strPtr(s.ref), strPtr(s.selector), strPtr(s.jsExpression)
			req.In = s.chain()
		} else {
			// A frame: the document the chain leads into.
			req.In = s.chainWithSelf()
		}
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
	resp, err := p.client.GetObservation(ctx, req)
	if err != nil {
		return "", err
	}
	if e := commandErrorFrom("getObservation", resp.GetError()); e != nil {
		return "", p.checked(ctx, o.Scope, e)
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
// @throws capture_failed - the page had no frame to copy. A page that has not
// produced one yet, or is not being composited at the moment, has nothing to
// hand over; retrying after it renders usually works
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	shot, err := page.Screenshot(ctx, "png", 0)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	img, _ := base64.StdEncoding.DecodeString(shot.DataBase64)
//	os.WriteFile("page.png", img, 0o644)
func (p *Page) Screenshot(ctx context.Context, format string, quality int32) (*ScreenshotResult, error) {
	resp, err := p.client.Screenshot(ctx, &generated.ScreenshotRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		Format:  strPtr(format),
		Quality: intPtr(quality),
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("screenshot", resp.GetError()); e != nil {
		return nil, e
	}
	return &ScreenshotResult{
		DataBase64: resp.DataBase64,
		Width:      resp.Width,
		Height:     resp.Height,
	}, nil
}
