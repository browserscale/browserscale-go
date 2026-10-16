package browserscale

import (
	"context"
	"errors"
	"fmt"

	"github.com/browserscale/browserscale-go/generated"
)

// WaitArg is the marker interface for everything Wait accepts: a condition
// (a [Locator], [Captcha], [Request] or [Response]) or a wait-level option
// such as [Timeout].
type WaitArg interface{ applyWait(*waitCall) }

type waitCall struct {
	conditions []waitCondition
	timeout    float64
	frameId    string
}

// waitCondition is one entry of a Wait call, in argument order. Exactly one
// field is set.
type waitCondition struct {
	locator  *Locator
	captcha  bool
	request  *RequestCondition
	response *ResponseCondition
}

// Locator implements WaitArg by appending itself to the call's condition
// list.
func (l *Locator) applyWait(w *waitCall) {
	w.conditions = append(w.conditions, waitCondition{locator: l})
}

// ── Wait-only conditions ──

// CaptchaCondition is the wait condition built by [Captcha]. It is accepted
// by [Page.Wait] only — it is not a [Locator], so no element action takes it.
type CaptchaCondition struct{}

func (c *CaptchaCondition) applyWait(w *waitCall) {
	w.conditions = append(w.conditions, waitCondition{captcha: true})
}

// Captcha matches as soon as a supported captcha or bot-protection challenge
// is on the page.
//
// It is a state, like [CSS]: a challenge that is already showing matches at
// once. On a match the result's Kind is [WaitKindCaptcha]; hand the page to
// [Page.SolveCaptcha] to solve it.
//
// Most challenges live in an iframe, so a wait containing Captcha searches
// every frame — its element conditions included — unless a locator pins a
// frame with [Locator.InFrame], which then scopes the captcha too.
//
// @returns *CaptchaCondition usable only as a [Page.Wait] condition
//
// @example
//
//	res, err := page.Wait(ctx,
//	    browserscale.CSS("#inbox"),
//	    browserscale.Captcha(),
//	)
//	if err == nil && res.Kind == browserscale.WaitKindCaptcha {
//	    _, err = page.SolveCaptcha(ctx, 0, 2)
//	}
func Captcha() *CaptchaCondition { return &CaptchaCondition{} }

// RequestCondition is the wait condition built by [Request]. It is accepted
// by [Page.Wait] only.
type RequestCondition struct {
	urlPattern string
	abort      bool
}

func (r *RequestCondition) applyWait(w *waitCall) {
	w.conditions = append(w.conditions, waitCondition{request: r})
}

// Abort drops the matching request: it is not sent, and the page receives an
// empty 200 instead. The request is still returned in the [WaitResult].
//
// @param v - true to drop the request, false to only observe it (the default)
//
// @returns the same RequestCondition for chaining
//
// @example
//
//	_, _ = page.Wait(ctx, browserscale.Request("https://tracker.example.com/*").Abort(true))
func (r *RequestCondition) Abort(v bool) *RequestCondition { r.abort = v; return r }

// Request matches the first request whose URL matches urlPattern.
//
// Unlike [CSS] or [Captcha], a request is an event, not a state: only a
// request issued after the wait started counts. Start the wait before the
// action that triggers the request — in a goroutine, or race it in the same
// call. It watches the whole browser session: the page, its frames, and any
// tab or popup it opens, across navigations.
//
// On a match the result's Kind is [WaitKindRequest] and Request carries the
// method, URL, headers and body.
//
// @param urlPattern - URL wildcard; `*` matches any character span
//
// @returns *RequestCondition usable only as a [Page.Wait] condition
//
// @example
//
//	res, err := page.Wait(ctx,
//	    browserscale.Request("https://example.com/api/submit*"),
//	    browserscale.Timeout(10000),
//	)
//	if err == nil {
//	    fmt.Println(res.Request.Method, res.Request.Body)
//	}
func Request(urlPattern string) *RequestCondition {
	return &RequestCondition{urlPattern: urlPattern}
}

// ResponseCondition is the wait condition built by [Response]. It is accepted
// by [Page.Wait] only.
type ResponseCondition struct {
	urlPattern string
	abort      bool
}

func (r *ResponseCondition) applyWait(w *waitCall) {
	w.conditions = append(w.conditions, waitCondition{response: r})
}

// Abort hides the matching response from the page: it receives an empty 200
// instead. The real response is still returned in the [WaitResult].
//
// @param v - true to hide the response, false to pass it through (the default)
//
// @returns the same ResponseCondition for chaining
//
// @example
//
//	_, _ = page.Wait(ctx, browserscale.Response("https://example.com/api/token*").Abort(true))
func (r *ResponseCondition) Abort(v bool) *ResponseCondition { r.abort = v; return r }

// Response matches the response to the first request whose URL matches
// urlPattern, once it has fully arrived.
//
// Same event semantics and scope as [Request]: only a request issued after
// the wait started counts, so start the wait before the action. On a match
// the result's Kind is [WaitKindResponse] and Response carries the status,
// headers and body.
//
// @param urlPattern - URL wildcard, matched against the request URL
//
// @returns *ResponseCondition usable only as a [Page.Wait] condition
//
// @example
//
//	res, err := page.Wait(ctx,
//	    browserscale.Response("https://example.com/api/token*"),
//	)
//	if err == nil {
//	    fmt.Println(res.Response.StatusCode, res.Response.Body)
//	}
func Response(urlPattern string) *ResponseCondition {
	return &ResponseCondition{urlPattern: urlPattern}
}

// ── Wait-only options ──

type waitOpt func(*waitCall)

func (f waitOpt) applyWait(w *waitCall) { f(w) }

// Timeout overrides the [Page.Wait] timeout.
//
// When omitted, the API's default of 30s applies. Pass once per Wait call as
// one of the variadic arguments.
//
// @param ms - timeout in milliseconds
//
// @returns a [WaitArg] suitable for passing to Wait
//
// @example
//
//	_, _ = page.Wait(ctx, browserscale.CSS("#done"), browserscale.Timeout(5000))
func Timeout(ms float64) WaitArg { return waitOpt(func(w *waitCall) { w.timeout = ms }) }

// ── Wait ──

// Wait blocks until any of the supplied conditions matches.
//
// Conditions can be mixed freely and are watched in parallel; the first one to
// match wins and the others are abandoned:
//   - [CSS] / [JS] — an element (or a truthy expression)
//   - [Captcha] — a supported captcha or bot-protection challenge
//   - [Request] / [Response] — a network request or its response
//
// Element and captcha conditions are states: they match if the page already
// is (or becomes) in that state. Request and response conditions are events:
// only what happens after the wait started counts. A wait does not block the
// page, so start it before the action that should trigger the event — run it
// in a goroutine alongside Click, SolveCaptcha or Navigate.
//
// Anything left unset is defaulted by the API, not by this SDK:
//   - timeout: 30s — override with [Timeout]
//   - per-locator visible and steady: visibility required, 500ms of settling.
//     For JS expressions returning a non-Element value (bool/string/number/
//     object) both are no-ops. Override with [Locator.Visible] /
//     [Locator.Steady] on individual locators.
//
// [Node] and [At] are not valid wait conditions — they only make sense as
// action targets — and produce an error at send time.
//
// @param args - one or more conditions plus optional wait-level options;
//
//	at least one condition is required
//
// @returns *WaitResult for the first matching condition: its Index (in
//
//	argument order), its Kind, and the matched Element, Request or Response
//
// If no condition matched before the timeout, err is a [*WaitError] carrying
// the failure code and a per-condition breakdown (Conditions) of why each
// never matched — e.g. "found_occluded" with the intercepting element, or
// "not_seen" for a request that never happened. The *WaitResult is still
// returned (with Index -1). Recover the detail with errors.As. A malformed
// call (no conditions, or an invalid condition) returns an ordinary error
// instead.
//
// @see [WaitError] for the timeout detail
//
// @example
//
//	// Whatever comes first: the inbox, a captcha, or the error page.
//	res, err := page.Wait(ctx,
//	    browserscale.CSS("#inbox"),
//	    browserscale.Captcha(),
//	    browserscale.Request("https://example.com/error*"),
//	    browserscale.Timeout(60000),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	switch res.Kind {
//	case browserscale.WaitKindElement:
//	    _, _ = page.Click(ctx, browserscale.Node(res.Element.BackendNodeId).InFrame(res.Element.FrameId))
//	case browserscale.WaitKindCaptcha:
//	    _, _ = page.SolveCaptcha(ctx, 0, 2)
//	case browserscale.WaitKindRequest:
//	    log.Println("failed:", res.Request.Url)
//	}
//
// @example
//
//	// Start the wait first, then trigger the request it waits for.
//	done := make(chan *browserscale.WaitResult, 1)
//	go func() {
//	    res, _ := page.Wait(ctx, browserscale.Response("https://example.com/api/login*"))
//	    done <- res
//	}()
//	_, _ = page.Click(ctx, browserscale.CSS("button[type=submit]"))
//	if res := <-done; res != nil && res.Response != nil {
//	    fmt.Println(res.Response.StatusCode)
//	}
func (p *Page) Wait(ctx context.Context, args ...WaitArg) (*WaitResult, error) {
	// timeout stays 0 unless Timeout() was passed, which leaves the field off the
	// wire so the API applies its own default.
	wc := &waitCall{}
	for _, a := range args {
		if a == nil {
			return nil, errors.New("browserscale.Wait: nil argument")
		}
		a.applyWait(wc)
	}
	if len(wc.conditions) == 0 {
		return nil, errors.New("browserscale.Wait: at least one condition required")
	}

	pbConds := make([]*generated.WaitCondition, len(wc.conditions))
	for i, cond := range wc.conditions {
		pc, err := cond.toProto(i)
		if err != nil {
			return nil, err
		}
		// If no call-level frameId was set, fall back to the first locator's
		// frameId. Captcha / request / response conditions have no frame.
		if l := cond.locator; l != nil && wc.frameId == "" && l.frameId != "" {
			wc.frameId = l.frameId
		}
		pbConds[i] = pc
	}

	req := &generated.WaitForAnyParams{
		SessionId:  p.sessionId,
		PageId:     p.id,
		ApiKey:     p.apiKey,
		Conditions: pbConds,
		Timeout:    floatPtrIfNonZero(wc.timeout),
	}
	if wc.frameId != "" {
		fid := wc.frameId
		req.FrameId = &fid
	}

	resp, err := p.client.WaitForAny(ctx, req)
	if err != nil {
		return nil, err
	}
	// A timeout (no condition matched) comes back as index=-1 with a structured
	// detail rather than a gRPC error. Surface it through err as a *WaitError so
	// the res, err shape stays identical to the other calls.
	res, waitErr := waitResultFromProto(resp)
	if waitErr != nil {
		return res, waitErr
	}
	return res, nil
}

func (c waitCondition) toProto(i int) (*generated.WaitCondition, error) {
	switch {
	case c.captcha:
		return &generated.WaitCondition{Captcha: true}, nil
	case c.request != nil:
		if c.request.urlPattern == "" {
			return nil, fmt.Errorf("browserscale.Wait: condition %d: Request needs a URL pattern", i)
		}
		return &generated.WaitCondition{Request: &generated.WaitNetworkCondition{
			UrlPattern: c.request.urlPattern, Abort: c.request.abort,
		}}, nil
	case c.response != nil:
		if c.response.urlPattern == "" {
			return nil, fmt.Errorf("browserscale.Wait: condition %d: Response needs a URL pattern", i)
		}
		return &generated.WaitCondition{Response: &generated.WaitNetworkCondition{
			UrlPattern: c.response.urlPattern, Abort: c.response.abort,
		}}, nil
	}
	l := c.locator
	if l == nil || (l.selector == "" && l.jsExpression == "") {
		return nil, fmt.Errorf("browserscale.Wait: condition %d has neither selector nor JS expression (Node()/At() are not valid wait conditions)", i)
	}
	pc := &generated.WaitCondition{}
	if l.selector != "" {
		s := l.selector
		pc.Selector = &s
	}
	if l.jsExpression != "" {
		e := l.jsExpression
		pc.JsExpression = &e
	}
	if l.visible != nil {
		v := *l.visible
		pc.Visible = &v
	}
	if l.steadyTime != nil {
		st := *l.steadyTime
		pc.SteadyTime = &st
	}
	return pc, nil
}
