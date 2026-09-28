package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// MoveTo moves the mouse cursor over the given target.
//
// The browser scrolls the target into view first if necessary, then animates
// the cursor along a human-like path to the element's random center area (or to the
// viewport coordinate when target is [At]).
//
// @param target - locator describing where to move; [At] is also valid
//
// @returns *ElementResult with the resolved frameId, backendNodeId,
//
//	post-scroll isVisible, element bounds and the root-viewport (rootX, rootY)
//	where the cursor ended up
//
// @throws not_found - no element matched the locator, so there was nowhere to
// move the cursor. The only semantic failure this call has: once the target is
// located, moving to it cannot be refused
//
// @see [MoveError] for recovering the code with errors.As
//
// @example
//
//	_, err := browser.MoveTo(ctx, browserscale.CSS("nav .menu"))
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) MoveTo(ctx context.Context, target *Locator) (*ElementResult, error) {
	if err := target.validateTarget("MoveTo", true); err != nil {
		return nil, err
	}

	resp, err := c.client.MoveTo(ctx, &generated.MoveToRequest{
		SessionId:     c.sessionId,
		ApiKey:        c.apiKey,
		Selector:      strPtr(target.selector),
		JsExpression:  strPtr(target.jsExpression),
		BackendNodeId: intPtr(target.backendNodeId),
		FrameId:       pickFrame("", target),
		X:             target.x,
		Y:             target.y,
	})
	if err != nil {
		return nil, err
	}
	// A target that could not be located comes back as success=false with a
	// structured detail rather than a gRPC error. Surface it through err as a
	// *MoveError.
	res, moveErr := moveResultFromProto(resp)
	if moveErr != nil {
		return res, moveErr
	}
	return res, nil
}
