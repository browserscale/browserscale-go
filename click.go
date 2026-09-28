package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// ClickOpts customizes a [CloudBrowser.ClickWith] call.
// Zero/empty values mean "use the server default".
type ClickOpts struct {
	// InFrame overrides the locator's own frame.
	// Empty = use the locator's frame (or the main frame if none).
	// Pass a specific frameId, or [AllFrames], to search elsewhere.
	InFrame string

	// Button is the mouse button to use.
	// Valid: "left" (default), "right", "middle".
	Button string

	// ClickCount controls single/double-click.
	// 0 or 1 = single click (default), 2 = double-click.
	ClickCount int32

	// Action selects the mouse phase.
	// "" or "click" = full mouseDown+mouseUp (default).
	// "press" only dispatches mouseDown.
	// "release" only dispatches mouseUp at the current cursor position.
	Action string
}

// Click triggers a single left mouse click on the given target.
//
// The element does not have to be ready when you call this. For up to 5s the
// browser keeps re-locating it, scrolls it into view, waits for its bounds to
// hold still for 750ms, and hit-tests the exact point it is about to press — so
// a plain Click also does the work of a preceding [CloudBrowser.Wait], and
// needs no retry loop of your own. If the element never settles within that
// budget the click is attempted at the deadline rather than abandoned.
//
// If something covers the target, the pointer is repositioned once to an
// exposed part of it, which also gives hover-triggered overlays a chance to
// collapse. Only if the target is still covered afterwards does the click
// refuse — it never presses whatever happens to lie on top.
//
// The cursor then moves along a human-like path rather than jumping, and a
// full mouseDown+mouseUp is dispatched at a randomized point inside the
// element's bounding rect.
//
// @param target - locator describing what to click; [At] is also valid
//
// @returns *ElementResult with success, resolved frameId, backendNodeId,
//
//	post-scroll isVisible, element bounds and the root-viewport (rootX, rootY)
//	where the click landed
//
// If the target was found but the click could not land because another element
// covered it, err is a [*ClickError] carrying the failure code and the
// intercepting element (Occluder). The *ElementResult is still returned (with
// Success=false) so callers can read the resolved coordinates. Recover the
// detail with errors.As.
//
// @throws not_found - no element matched the locator, or it could not be scrolled
// into view
// @throws occluded_no_reachable_point - the target is fully covered, with no
// exposed part left to click
// @throws occluded_after_evade - a reposition was tried and the target was still
// covered
//
// A target that is empty or names several things at once is rejected before
// anything is sent. A closed page or a frame that is gone is a transport failure
// rather than a code.
//
// @see [ClickError] for the occlusion-failure detail
// @see [CloudBrowser.ClickWith] for right-click, double-click,
//
//	press/release-only, or frame override
//
// @example
//
//	res, err := browser.Click(ctx, browserscale.CSS("button.submit"))
//	if err != nil {
//	    var ce *browserscale.ClickError
//	    if errors.As(err, &ce) {
//	        log.Printf("blocked by %s (%s)", ce.Occluder.TagName, ce.Code)
//	    }
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) Click(ctx context.Context, target *Locator) (*ElementResult, error) {
	return c.clickWith(ctx, target, ClickOpts{})
}

// ClickWith is the customizable variant of [CloudBrowser.Click].
//
// @inheritDoc [CloudBrowser.Click]
// @param opts - click customization; see [ClickOpts]
//
// @example
//
//	// Right double-click on a context menu trigger.
//	_, err := browser.ClickWith(ctx, browserscale.CSS("li.menu"), browserscale.ClickOpts{
//	    Button:     "right",
//	    ClickCount: 2,
//	})
func (c *CloudBrowser) ClickWith(ctx context.Context, target *Locator, opts ClickOpts) (*ElementResult, error) {
	return c.clickWith(ctx, target, opts)
}

func (c *CloudBrowser) clickWith(ctx context.Context, target *Locator, o ClickOpts) (*ElementResult, error) {
	if err := target.validateTarget("Click", true); err != nil {
		return nil, err
	}

	resp, err := c.client.Click(ctx, &generated.ClickRequest{
		SessionId:     c.sessionId,
		ApiKey:        c.apiKey,
		Selector:      strPtr(target.selector),
		JsExpression:  strPtr(target.jsExpression),
		BackendNodeId: intPtr(target.backendNodeId),
		FrameId:       pickFrame(o.InFrame, target),
		X:             target.x,
		Y:             target.y,
		Button:        strPtr(o.Button),
		ClickCount:    intPtr(o.ClickCount),
		Action:        strPtr(o.Action),
	})
	if err != nil {
		return nil, err
	}
	// A click that did not land comes back as success=false with a structured
	// detail rather than a gRPC error. Surface it through err as a *ClickError
	// so the res, err shape stays identical to the other actions.
	res, clickErr := clickResultFromProto(resp)
	if clickErr != nil {
		return res, clickErr
	}
	return res, nil
}
