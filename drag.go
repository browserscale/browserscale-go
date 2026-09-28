package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// DragBy picks up the target and drops it at an offset relative to the
// pickup point.
//
// The source is acquired with the same smart click as [CloudBrowser.Click] —
// re-located, scrolled into view, settled and hit-tested — so the handle does
// not have to be ready when you call this. The browser then presses the left
// mouse button at a point inside the element, drags along a human-like path to
// (pickupX+offsetX, pickupY+offsetY), and releases. [At] is not a valid
// target — drag needs a real element.
//
// @param target - locator describing the element to pick up
// @param offsetX - horizontal distance to drag, in CSS pixels
// @param offsetY - vertical distance to drag, in CSS pixels
//
// @returns *DragResult with the resolved frameId, backendNodeId and the
//
//	final cursor position (rootX, rootY) where the drop happened
//
// If the pickup itself failed — the handle was not found, or something covered
// it — err is a [*DragError] whose ClickError carries the full occlusion
// detail. Recover it with errors.As.
//
// @throws not_found - no element matched the locator
// @throws occluded_no_reachable_point - the handle is fully covered, with no
// exposed part left to grab
// @throws occluded_after_evade - a reposition was tried and the handle was still
// covered
//
// @see [DragError] for the occlusion-failure detail
//
// @example
//
//	_, err := browser.DragBy(ctx, browserscale.CSS(".slider .handle"), 120, 0)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) DragBy(ctx context.Context, target *Locator, offsetX, offsetY float64) (*DragResult, error) {
	return c.drag(ctx, target, &offsetX, &offsetY, nil, nil)
}

// DragTo picks up the target and drops it at absolute root-viewport
// coordinates.
//
// Same gesture and same source acquisition as [CloudBrowser.DragBy], but the
// drop destination is in page coordinates rather than relative to the pickup
// point.
//
// @param target - locator describing the element to pick up
// @param absoluteX - horizontal drop coordinate in the root viewport
// @param absoluteY - vertical drop coordinate in the root viewport
//
// @returns *DragResult with the resolved frameId, backendNodeId and the
//
//	final cursor position (rootX, rootY) where the drop happened
//
// @throws not_found - no element matched the locator
// @throws occluded_no_reachable_point - the handle is fully covered, with no
// exposed part left to grab
// @throws occluded_after_evade - a reposition was tried and the handle was still
// covered
//
// @see [DragError] for the occlusion-failure detail
//
// @example
//
//	_, err := browser.DragTo(ctx, browserscale.CSS(".card"), 800, 400)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) DragTo(ctx context.Context, target *Locator, absoluteX, absoluteY float64) (*DragResult, error) {
	return c.drag(ctx, target, nil, nil, &absoluteX, &absoluteY)
}

func (c *CloudBrowser) drag(ctx context.Context, target *Locator, ox, oy, ax, ay *float64) (*DragResult, error) {
	if err := target.validateTarget("Drag", false); err != nil {
		return nil, err
	}
	resp, err := c.client.Drag(ctx, &generated.DragRequest{
		SessionId:     c.sessionId,
		ApiKey:        c.apiKey,
		Selector:      strPtr(target.selector),
		JsExpression:  strPtr(target.jsExpression),
		BackendNodeId: intPtr(target.backendNodeId),
		FrameId:       pickFrame("", target),
		OffsetX:       ox,
		OffsetY:       oy,
		AbsoluteX:     ax,
		AbsoluteY:     ay,
	})
	if err != nil {
		return nil, err
	}
	// A failed source pickup comes back as success=false with a structured
	// detail rather than a gRPC error. Surface it through err as a *DragError.
	res, dragErr := dragResultFromProto(resp)
	if dragErr != nil {
		return res, dragErr
	}
	return res, nil
}
