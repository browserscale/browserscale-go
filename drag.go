package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// DragBy picks up the target and drops it at an offset relative to the
// pickup point.
//
// The browser presses the left mouse button at a pickup point inside the
// element, drags along a human-like path to (pickupX+offsetX,
// pickupY+offsetY), then releases. [At] is not a valid target — drag
// needs a real element.
//
// @param target - locator describing the element to pick up
// @param offsetX - horizontal distance to drag, in CSS pixels
// @param offsetY - vertical distance to drag, in CSS pixels
//
// @returns *DragResult with the resolved frameId, backendNodeId and the
//
//	final cursor position (rootX, rootY) where the drop happened
//
// @throws UNKNOWN_ERROR - the drag could not be performed
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
// Same gesture as [CloudBrowser.DragBy], but the drop destination is in
// page coordinates rather than relative to the pickup point.
//
// @param target - locator describing the element to pick up
// @param absoluteX - horizontal drop coordinate in the root viewport
// @param absoluteY - vertical drop coordinate in the root viewport
//
// @returns *DragResult with the resolved frameId, backendNodeId and the
//
//	final cursor position (rootX, rootY) where the drop happened
//
// @throws UNKNOWN_ERROR - the drag could not be performed
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
	return dragResultFromProto(resp), nil
}
