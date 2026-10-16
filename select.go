package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// SelectOpts customizes a SelectByXxxWith call.
// Zero/empty values mean "use the server default".
type SelectOpts struct {
	// InFrame overrides the locator's own frame.
	// Empty = use the locator's frame (or the main frame if none).
	// Pass a specific frameId, or [AllFrames], to search elsewhere.
	InFrame string

	// NoEvents picks the option silently without firing input/change
	// events. Default (false) fires the standard events.
	NoEvents bool
}

// SelectByIndex picks the <option> at the zero-based index inside the
// targeted <select> element.
//
// Sets the option as selected on the targeted <select>, then fires the
// standard input + change events (unless suppressed via
// [Page.SelectByIndexWith] with [SelectOpts].NoEvents).
//
// [At] is not a valid target — Select requires an actual <select>
// element.
//
// @param target - locator describing the <select> element
// @param index - zero-based option index
//
// @returns *SelectOptionResult with the resolved selectedIndex,
//
//	selectedValue and selectedText after the change
//
// @throws not_found - the <select> element could not be located
// @throws option_not_found - the <select> was found but no option matched, so the
// index is out of range or the element is not a <select> at all
//
// selectOption is programmatic and has no pointer gate, so there is no occlusion
// code here. A target that is empty or names several things at once is rejected
// before anything is sent, and a closed page or a frame that is gone is a
// transport failure rather than a code.
//
// @see [SelectOptionError] for recovering the code with errors.As
//
// @see [Page.SelectByIndexWith] for suppressing events or
//
//	overriding the target frame
//
// @see [Page.SelectByValue], [Page.SelectByText]
//
//	for matching by `value` attribute or visible text instead
//
// @example
//
//	_, err := page.SelectByIndex(ctx, browserscale.CSS("select#country"), 2)
func (p *Page) SelectByIndex(ctx context.Context, target *Locator, index int32) (*SelectOptionResult, error) {
	return p.selectByIndexWith(ctx, target, index, SelectOpts{})
}

// SelectByIndexWith is the customizable variant of [Page.SelectByIndex].
//
// @inheritDoc [Page.SelectByIndex]
// @param opts - select customization; see [SelectOpts]
//
// @example
//
//	// Pick the option silently, no input/change events.
//	_, err := page.SelectByIndexWith(ctx, browserscale.CSS("select#hidden"), 0, browserscale.SelectOpts{
//	    NoEvents: true,
//	})
func (p *Page) SelectByIndexWith(ctx context.Context, target *Locator, index int32, opts SelectOpts) (*SelectOptionResult, error) {
	return p.selectByIndexWith(ctx, target, index, opts)
}

func (p *Page) selectByIndexWith(ctx context.Context, target *Locator, index int32, o SelectOpts) (*SelectOptionResult, error) {
	i := index
	return p.runSelect(ctx, target, o, func(req *generated.SelectOptionRequest) { req.Index = &i })
}

// SelectByValue picks the <option> whose `value` attribute matches the
// given string exactly.
//
// @inheritDoc [Page.SelectByIndex]
// @param value - the `value` attribute to match
//
// @example
//
//	_, err := page.SelectByValue(ctx, browserscale.CSS("select#country"), "DE")
func (p *Page) SelectByValue(ctx context.Context, target *Locator, value string) (*SelectOptionResult, error) {
	return p.selectByValueWith(ctx, target, value, SelectOpts{})
}

// SelectByValueWith is the customizable variant of [Page.SelectByValue].
//
// @inheritDoc [Page.SelectByValue]
// @param opts - select customization; see [SelectOpts]
//
// @example
//
//	_, err := page.SelectByValueWith(ctx, browserscale.CSS("select#country"), "DE", browserscale.SelectOpts{
//	    NoEvents: true,
//	})
func (p *Page) SelectByValueWith(ctx context.Context, target *Locator, value string, opts SelectOpts) (*SelectOptionResult, error) {
	return p.selectByValueWith(ctx, target, value, opts)
}

func (p *Page) selectByValueWith(ctx context.Context, target *Locator, value string, o SelectOpts) (*SelectOptionResult, error) {
	v := value
	return p.runSelect(ctx, target, o, func(req *generated.SelectOptionRequest) { req.Value = &v })
}

// SelectByText picks the <option> whose visible (trimmed) text matches
// the given string exactly.
//
// @inheritDoc [Page.SelectByIndex]
// @param text - the visible option text to match
//
// @example
//
//	_, err := page.SelectByText(ctx, browserscale.CSS("select#country"), "Germany")
func (p *Page) SelectByText(ctx context.Context, target *Locator, text string) (*SelectOptionResult, error) {
	return p.selectByTextWith(ctx, target, text, SelectOpts{})
}

// SelectByTextWith is the customizable variant of [Page.SelectByText].
//
// @inheritDoc [Page.SelectByText]
// @param opts - select customization; see [SelectOpts]
//
// @example
//
//	_, err := page.SelectByTextWith(ctx, browserscale.CSS("select#country"), "Germany", browserscale.SelectOpts{
//	    NoEvents: true,
//	})
func (p *Page) SelectByTextWith(ctx context.Context, target *Locator, text string, opts SelectOpts) (*SelectOptionResult, error) {
	return p.selectByTextWith(ctx, target, text, opts)
}

func (p *Page) selectByTextWith(ctx context.Context, target *Locator, text string, o SelectOpts) (*SelectOptionResult, error) {
	t := text
	return p.runSelect(ctx, target, o, func(req *generated.SelectOptionRequest) { req.Text = &t })
}

func (p *Page) runSelect(ctx context.Context, target *Locator, o SelectOpts, withKey func(*generated.SelectOptionRequest)) (*SelectOptionResult, error) {
	if err := target.validateTarget("SelectOption", false); err != nil {
		return nil, err
	}
	req := &generated.SelectOptionRequest{
		SessionId:     p.sessionId,
		PageId:        p.id,
		ApiKey:        p.apiKey,
		Selector:      strPtr(target.selector),
		JsExpression:  strPtr(target.jsExpression),
		BackendNodeId: intPtr(target.backendNodeId),
		FrameId:       pickFrame(o.InFrame, target),
	}
	withKey(req)
	if o.NoEvents {
		f := false
		req.FireEvents = &f
	}
	resp, err := p.client.SelectOption(ctx, req)
	if err != nil {
		return nil, err
	}
	// A select that matched nothing comes back as success=false with a
	// structured detail rather than a gRPC error. Surface it through err as a
	// *SelectOptionError.
	res, selErr := selectOptionResultFromProto(resp)
	if selErr != nil {
		return res, selErr
	}
	return res, nil
}
