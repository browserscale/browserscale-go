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
// @returns *ElementResult with the the element's Ref and Frame,
//
//	post-scroll isVisible and the element's bounds after the scroll
//
// @throws container_not_found - a container given with [Locator.In] is not on
// the page; the message names the link of the chain that was missing
// @throws outside_scope - a [JS] target returned an element outside the element
// it was confined to with [Locator.In]
// @throws [StaleRefError] - the target or a container is a reference whose
// element is gone (code "stale_ref")
// @throws not_found - no element matched the locator, or it could not be
// scrolled. The only semantic failure this call has
//
// @see [ScrollError] for recovering the code with errors.As
//
// @example
//
//	_, err := page.ScrollTo(ctx, browserscale.CSS("#footer"))
//	if err != nil {
//	    log.Fatal(err)
//	}
func (p *Page) ScrollTo(ctx context.Context, target *Locator) (*ElementResult, error) {
	if err := target.validateTarget("ScrollTo", false); err != nil {
		return nil, err
	}

	resp, err := p.client.ScrollTo(ctx, &generated.ScrollToRequest{
		SessionId:    p.sessionId,
		PageId:       p.id,
		ApiKey:       p.apiKey,
		Ref:          strPtr(target.ref),
		Selector:     strPtr(target.selector),
		JsExpression: strPtr(target.jsExpression),
		In:           target.chain(),
	})
	if err != nil {
		return nil, err
	}
	// A target that could not be located/scrolled comes back as success=false
	// with a structured detail rather than a gRPC error. Surface it through err
	// as a *ScrollError.
	res, scrollErr := scrollResultFromProto(resp)
	if scrollErr != nil {
		return res, p.checked(ctx, target, scrollErr)
	}
	return res, nil
}
