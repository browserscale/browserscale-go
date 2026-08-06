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
}

// Fill clicks the target and types text into it, appending to any
// existing content.
//
// The browser scrolls the element into view, moves the cursor along a
// human-like path, clicks to focus, then types the text character-by-
// character with QWERTZ keyboard simulation and human-like timing.
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
// @throws INVALID_LOCATOR - target is empty or has multiple targets set
// @throws TIMEOUT - the operation exceeded the server-side timeout
// @throws PAGE_NOT_ALIVE - the page has been closed
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
