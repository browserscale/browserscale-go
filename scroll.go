package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// ScrollTo scrolls the given element into view.
//
// Whatever scroll container is closest to the element does the scrolling —
// nested scroll containers and out-of-process iframe chains are walked
// automatically. [At] is not a valid target here; scrolling needs a real
// element.
//
// @param target - locator describing the element to bring into view;
//
//	[At] is rejected
//
// @returns *ElementResult with the resolved frameId, backendNodeId,
//
//	post-scroll isVisible and the element's bounds after the scroll
//
// @throws UNKNOWN_ERROR - the element could not be scrolled into view
//
// @example
//
//	_, err := browser.ScrollTo(ctx, browserscale.CSS("#footer"))
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) ScrollTo(ctx context.Context, target *Locator) (*ElementResult, error) {
	if err := target.validateTarget("ScrollTo", false); err != nil {
		return nil, err
	}

	resp, err := c.client.ScrollTo(ctx, &generated.ScrollToRequest{
		SessionId:     c.sessionId,
		ApiKey:        c.apiKey,
		Selector:      strPtr(target.selector),
		JsExpression:  strPtr(target.jsExpression),
		BackendNodeId: intPtr(target.backendNodeId),
		FrameId:       pickFrame("", target),
	})
	if err != nil {
		return nil, err
	}
	return elementResultFromProto(resp), nil
}
