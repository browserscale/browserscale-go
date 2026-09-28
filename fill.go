package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// FillOpts customizes a [CloudBrowser.FillWith] call.
// Zero/empty values mean "use the server default".
type FillOpts struct {
	// InFrame overrides the locator's own frame.
	// Empty = use the locator's frame (or the main frame if none).
	// Pass a specific frameId, or [AllFrames], to search elsewhere.
	InFrame string

	// ClearFirst, when true, wipes the field's existing content with
	// Ctrl+A, Delete before typing. Default (false) appends to whatever
	// is already there.
	ClearFirst bool

	// TimeoutMs bounds focus acquisition (locate, scroll, settle, un-occlude)
	// in ms, mirroring the click timeout. nil = server default (5000). It is a
	// pointer because 0 is meaningful: browserscale.Ptr(0.0) makes Fill
	// one-shot (no retry).
	TimeoutMs *float64

	// SteadyMs is the settle window in ms before the focus click, mirroring the
	// click steady-time. nil = server default (750); browserscale.Ptr(0.0)
	// skips settling.
	SteadyMs *float64
}

// Fill clicks the target and types text into it, appending to any
// existing content.
//
// The field is acquired with the same smart click as [CloudBrowser.Click]:
// re-located, scrolled into view, settled and hit-tested within the timeout
// budget, so it does not have to be present or ready yet. The cursor then
// moves along a human-like path and clicks to focus.
//
// Typing is per-key rather than a value assignment: keyDown, char and keyUp for
// every character, with the keycodes of the layout that matches the session's
// region and human cadence between them. If the field already holds text the
// caret is moved to the end first, so appended input lands after the existing
// content instead of wherever the caret happened to sit.
//
// Fill is strictly target-bound. If something else takes focus mid-typing, the
// remaining characters are never typed into the thief — the browser tries to
// re-focus the target and otherwise fails with "focus_stolen", naming the
// element that holds focus instead so you can deal with it (a consent button,
// a different field). For stream-style typing that is *supposed* to move
// between fields, such as an OTP input that auto-advances, use
// [CloudBrowser.Type] instead.
//
// To overwrite the field instead of appending, use [CloudBrowser.FillWith]
// with ClearFirst: true.
//
// [At] is not a valid target — Fill requires an actual element.
//
// @param target - locator describing the input element
// @param text - text to type into the element
//
// @returns *ElementResult with success, resolved frameId, backendNodeId
//
//	and the root-viewport (rootX, rootY) where the element was clicked
//
// Fill focuses the field with the same smart click as [CloudBrowser.Click], so
// if the field could not be focused (not found, or occluded), err is a
// [*FillError] whose ClickError carries the underlying occlusion detail. The
// *ElementResult is still returned (with Success=false). Recover the detail
// with errors.As.
//
// @throws not_found - no element matched the locator, or it could not be scrolled
// into view
// @throws occluded_no_reachable_point - the field is fully covered, with no
// exposed part left to click
// @throws occluded_after_evade - a reposition was tried and the field was still
// covered
// @throws focus_stolen - typing had started and another element took focus. The
// detail names what holds it, which is usually the overlay or autocomplete popup
// that interrupted
// @throws focus_lost - focus left the field and nothing holds it anymore, so the
// field vanished or turned readonly mid-stream
//
// A target that is empty or names several things at once is rejected before
// anything is sent. A closed page or a frame that is gone is a transport failure
// rather than a code.
//
// @see [FillError] for the focus-failure detail
// @see [CloudBrowser.FillWith] for clearing existing content or
//
//	overriding the target frame
//
// @example
//
//	res, err := browser.Fill(ctx, browserscale.CSS("input[name=email]"), "user@example.com")
//	if err != nil {
//	    var fe *browserscale.FillError
//	    if errors.As(err, &fe) && fe.ClickError != nil {
//	        log.Printf("blocked by %s", fe.ClickError.Occluder.TagName)
//	    }
//	    log.Fatal(err)
//	}
//	_ = res
func (c *CloudBrowser) Fill(ctx context.Context, target *Locator, text string) (*ElementResult, error) {
	return c.fillWith(ctx, target, text, FillOpts{})
}

// FillWith is the customizable variant of [CloudBrowser.Fill].
//
// @inheritDoc [CloudBrowser.Fill]
// @param opts - fill customization; see [FillOpts]
//
// @example
//
//	// Wipe the field first, then type fresh content.
//	_, err := browser.FillWith(ctx, browserscale.CSS("input[name=email]"), "user@example.com", browserscale.FillOpts{
//	    ClearFirst: true,
//	})
func (c *CloudBrowser) FillWith(ctx context.Context, target *Locator, text string, opts FillOpts) (*ElementResult, error) {
	return c.fillWith(ctx, target, text, opts)
}

func (c *CloudBrowser) fillWith(ctx context.Context, target *Locator, text string, o FillOpts) (*ElementResult, error) {
	if err := target.validateTarget("Fill", false); err != nil {
		return nil, err
	}

	req := &generated.FillRequest{
		SessionId:     c.sessionId,
		ApiKey:        c.apiKey,
		Selector:      strPtr(target.selector),
		JsExpression:  strPtr(target.jsExpression),
		BackendNodeId: intPtr(target.backendNodeId),
		FrameId:       pickFrame(o.InFrame, target),
		Text:          text,
	}
	if o.ClearFirst {
		t := true
		req.ClearFirst = &t
	}
	req.Timeout = o.TimeoutMs
	req.SteadyTime = o.SteadyMs

	resp, err := c.client.Fill(ctx, req)
	if err != nil {
		return nil, err
	}
	// A field that could not be focused/typed comes back as success=false with
	// a structured detail rather than a gRPC error. Surface it through err as a
	// *FillError so the res, err shape stays identical to the other actions.
	res, fillErr := fillResultFromProto(resp)
	if fillErr != nil {
		return res, fillErr
	}
	return res, nil
}
