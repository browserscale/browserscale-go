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
// The browser scrolls the element into view if needed, moves the cursor
// along a human-like path, then dispatches a full mouseDown+mouseUp at a
// randomized point inside the element's bounding rect.
//
// @param target - locator describing what to click; [At] is also valid
//
// @returns *ElementResult with success, resolved frameId, backendNodeId,
//
//	post-scroll isVisible, element bounds and the root-viewport (rootX, rootY)
//	where the click landed
//
// @throws INVALID_LOCATOR - target is empty or has multiple targets set
// @throws ELEMENT_NOT_FOUND - no element matched the locator
// @throws FRAME_NOT_FOUND - the requested frame does not exist
// @throws CLICK_FAILED - the click could not be dispatched
// @throws TIMEOUT - the operation exceeded the server-side timeout
// @throws PAGE_NOT_ALIVE - the page has been closed
//
// @see [CloudBrowser.ClickWith] for right-click, double-click,
//
//	press/release-only, or frame override
//
// @example
//
//	_, err := browser.Click(ctx, browserscale.CSS("button.submit"))
//	if err != nil {
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
	return elementResultFromProto(resp), nil
}
