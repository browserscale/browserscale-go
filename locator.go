package browserscale

import "fmt"

// AllFrames is the sentinel for "match in every frame on the page".
// Use it for any field documented as accepting a frameId — both
// Locator.InFrame and XxxOpts.InFrame.
const AllFrames = "ALL_FRAMES"

// Locator is the universal "what element / what condition" type. It is used
// both as a wait condition (passed to Wait) and as a target for element
// actions (passed to Click, Fill, etc.).
//
// Not every field is meaningful in every context:
//   - selector / jsExpression  → both wait and actions
//   - backendNodeId            → actions only (Wait rejects it)
//   - visible / steadyTime     → wait only (silently ignored by actions)
//   - x / y                    → actions only (Wait rejects it)
//   - frameId                  → both, may be overridden by call-level
//     browserscale.InFrame() / browserscale.InAllFrames() options
//
// Use the CSS / JS / Node / At constructors instead of building this struct
// by hand.
type Locator struct {
	selector      string
	jsExpression  string
	backendNodeId int32
	frameId       string

	visible    *bool
	steadyTime *float64

	x *float64
	y *float64
}

// CSS waits for / targets an element matching the given CSS selector.
//
// When used in [CloudBrowser.Wait], the returned Locator carries the SDK
// defaults [DefaultVisible] (true) and [DefaultSteadyMs] (500). Override
// per call with [Locator.Visible] / [Locator.Steady] (use `.Steady(0)` to
// disable the steady check).
//
// When used as an action target (Click, etc.) the visible/steady fields
// are ignored — there are no corresponding fields on the action requests.
//
// @param selector - CSS selector matching the element
//
// @returns *Locator usable as a wait condition or as an action target
//
// @example
//
//	// As a wait condition.
//	_, _ = browser.Wait(ctx, browserscale.CSS("button.submit"))
//	// As an action target.
//	_, _ = browser.Click(ctx, browserscale.CSS("button.submit"))
func CSS(selector string) *Locator {
	v := DefaultVisible
	st := DefaultSteadyMs
	return &Locator{selector: selector, visible: &v, steadyTime: &st}
}

// JS waits for / targets the result of a JavaScript expression.
//
// Same wait defaults as [CSS] ([DefaultVisible]=true, [DefaultSteadyMs]=500);
// these only apply when the expression returns a DOM Element. For non-Element
// truthy values (boolean, string, number, plain object) both fields are no-ops
// and the condition matches as soon as the value is truthy.
//
// Use [Locator.Visible](false) / [Locator.Steady](0) on the returned Locator
// to opt out.
//
// @param expression - JavaScript expression evaluated in the target frame
//
// @returns *Locator usable as a wait condition or as an action target
//
// @example
//
//	_, _ = browser.Wait(ctx, browserscale.JS("window.__ready === true"))
func JS(expression string) *Locator {
	v := DefaultVisible
	st := DefaultSteadyMs
	return &Locator{jsExpression: expression, visible: &v, steadyTime: &st}
}

// Node targets an element by its DevTools backendNodeId.
//
// Use this when you already have a backendNodeId from a previous result
// (e.g. [WaitResult] or [EvaluateResult]) and want to act on the exact same
// element without re-resolving by selector. Action-only — using it in
// [CloudBrowser.Wait] returns an error at send time.
//
// @param backendNodeId - DevTools backendNodeId of the target element
//
// @returns *Locator usable only as an action target
//
// @example
//
//	res, _ := browser.Click(ctx, browserscale.CSS("button.open"))
//	_, _ = browser.Click(ctx, browserscale.Node(res.BackendNodeId))
func Node(backendNodeId int32) *Locator {
	return &Locator{backendNodeId: backendNodeId}
}

// At targets viewport coordinates instead of an element.
//
// Useful for clicking inside a canvas, hovering decorative regions, or
// dispatching events at synthetic positions. Action-only — using it in
// [CloudBrowser.Wait] returns an error at send time. Note that only Click
// and MoveTo accept At; Scroll, Drag, Fill and Select all require a real
// element.
//
// @param x - viewport-relative x in CSS pixels
// @param y - viewport-relative y in CSS pixels
//
// @returns *Locator usable only as an action target
//
// @example
//
//	// Click at canvas-relative coordinates.
//	_, _ = browser.Click(ctx, browserscale.At(120, 240))
func At(x, y float64) *Locator {
	return &Locator{x: &x, y: &y}
}

// ── Locator modifiers (per-locator) ──

// Visible enforces or disables the visibility check for this Locator's
// wait condition.
//
// Pass false to opt out of the default [DefaultVisible] (true). Has no
// effect when the Locator is used as an action target — actions never
// check visibility before dispatching.
//
// @param v - true to require visibility, false to skip the check
//
// @returns the same Locator for chaining
//
// @example
//
//	_, _ = browser.Wait(ctx, browserscale.CSS("#hidden").Visible(false))
func (l *Locator) Visible(v bool) *Locator { l.visible = &v; return l }

// Steady requires the element to keep a stable position and size for at
// least ms milliseconds before the wait matches.
//
// Pass 0 to disable the default [DefaultSteadyMs] (500). Has no effect for
// JS expressions that return a non-Element value, nor when the Locator is
// used as an action target.
//
// @param ms - steady-state duration in milliseconds; 0 disables
//
// @returns the same Locator for chaining
//
// @example
//
//	_, _ = browser.Wait(ctx, browserscale.CSS(".banner").Steady(0))
func (l *Locator) Steady(ms float64) *Locator { l.steadyTime = &ms; return l }

// InFrame scopes this Locator to a specific frameId.
//
// Use the frameId from a previous result or [CloudBrowser.GetPages] to
// target elements inside a known iframe.
//
// @param id - id of the frame to scope to
//
// @returns the same Locator for chaining
//
// @example
//
//	pages, _ := browser.GetPages(ctx)
//	iframeId := pages[0].FrameTree.Children[0].FrameId
//	_, _ = browser.Click(ctx, browserscale.CSS("button").InFrame(iframeId))
func (l *Locator) InFrame(id string) *Locator { l.frameId = id; return l }

// InAllFrames scopes this Locator to every frame.
//
// Equivalent to `.InFrame(AllFrames)`. Use this when an element might
// appear inside any of several frames and you do not want to enumerate
// them.
//
// @returns the same Locator for chaining
//
// @example
//
//	_, _ = browser.Wait(ctx, browserscale.CSS("button.consent").InAllFrames())
func (l *Locator) InAllFrames() *Locator { l.frameId = "ALL_FRAMES"; return l }

// validateTarget validates the locator for an element-action call.
// allowCoords controls whether an At(x,y) locator is acceptable for this
// command (Click and MoveTo accept it; ScrollTo, Drag, SelectOption, Fill
// require an actual element).
func (l *Locator) validateTarget(cmd string, allowCoords bool) error {
	if l == nil {
		return fmt.Errorf("browserscale.%s: a target locator is required", cmd)
	}
	hasCoords := l.x != nil && l.y != nil
	hasElement := l.selector != "" || l.jsExpression != "" || l.backendNodeId != 0
	switch {
	case !hasElement && !hasCoords:
		return fmt.Errorf("browserscale.%s: target must have selector, JS expression, backendNodeId, or At(x,y)", cmd)
	case hasCoords && !allowCoords:
		return fmt.Errorf("browserscale.%s: At(x,y) is not supported here — use an element locator", cmd)
	}
	return nil
}

// pickFrame returns the frame to send on the wire: opts.InFrame wins over
// the locator's own frameId. Empty everywhere → nil (server uses main frame).
func pickFrame(optsFrame string, locator *Locator) *string {
	switch {
	case optsFrame != "":
		f := optsFrame
		return &f
	case locator != nil && locator.frameId != "":
		f := locator.frameId
		return &f
	}
	return nil
}
