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
// [CloudBrowser.SelectByIndexWith] with [SelectOpts].NoEvents).
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
// @throws INVALID_LOCATOR - target is empty or has multiple targets set
// @throws ELEMENT_NOT_FOUND - no element matched the locator
// @throws FRAME_NOT_FOUND - the requested frame does not exist
// @throws SELECT_FAILED - the option could not be selected
//
//	(out of range, or element is not a <select>)
//
// @throws TIMEOUT - the operation exceeded the server-side timeout
// @throws PAGE_NOT_ALIVE - the page has been closed
//
// @see [CloudBrowser.SelectByIndexWith] for suppressing events or
//
//	overriding the target frame
//
// @see [CloudBrowser.SelectByValue], [CloudBrowser.SelectByText]
//
//	for matching by `value` attribute or visible text instead
//
// @example
//
//	_, err := browser.SelectByIndex(ctx, browserscale.CSS("select#country"), 2)
func (c *CloudBrowser) SelectByIndex(ctx context.Context, target *Locator, index int32) (*SelectOptionResult, error) {
	return c.selectByIndexWith(ctx, target, index, SelectOpts{})
}

// SelectByIndexWith is the customizable variant of [CloudBrowser.SelectByIndex].
//
// @inheritDoc [CloudBrowser.SelectByIndex]
// @param opts - select customization; see [SelectOpts]
//
// @example
//
//	// Pick the option silently, no input/change events.
//	_, err := browser.SelectByIndexWith(ctx, browserscale.CSS("select#hidden"), 0, browserscale.SelectOpts{
//	    NoEvents: true,
//	})
func (c *CloudBrowser) SelectByIndexWith(ctx context.Context, target *Locator, index int32, opts SelectOpts) (*SelectOptionResult, error) {
	return c.selectByIndexWith(ctx, target, index, opts)
}

func (c *CloudBrowser) selectByIndexWith(ctx context.Context, target *Locator, index int32, o SelectOpts) (*SelectOptionResult, error) {
	i := index
	return c.runSelect(ctx, target, o, func(req *generated.SelectOptionRequest) { req.Index = &i })
}

// SelectByValue picks the <option> whose `value` attribute matches the
// given string exactly.
//
// @inheritDoc [CloudBrowser.SelectByIndex]
// @param value - the `value` attribute to match
//
// @example
//
//	_, err := browser.SelectByValue(ctx, browserscale.CSS("select#country"), "DE")
func (c *CloudBrowser) SelectByValue(ctx context.Context, target *Locator, value string) (*SelectOptionResult, error) {
	return c.selectByValueWith(ctx, target, value, SelectOpts{})
}

// SelectByValueWith is the customizable variant of [CloudBrowser.SelectByValue].
//
// @inheritDoc [CloudBrowser.SelectByValue]
// @param opts - select customization; see [SelectOpts]
//
// @example
//
//	_, err := browser.SelectByValueWith(ctx, browserscale.CSS("select#country"), "DE", browserscale.SelectOpts{
//	    NoEvents: true,
//	})
func (c *CloudBrowser) SelectByValueWith(ctx context.Context, target *Locator, value string, opts SelectOpts) (*SelectOptionResult, error) {
	return c.selectByValueWith(ctx, target, value, opts)
}

func (c *CloudBrowser) selectByValueWith(ctx context.Context, target *Locator, value string, o SelectOpts) (*SelectOptionResult, error) {
	v := value
	return c.runSelect(ctx, target, o, func(req *generated.SelectOptionRequest) { req.Value = &v })
}

// SelectByText picks the <option> whose visible (trimmed) text matches
// the given string exactly.
//
// @inheritDoc [CloudBrowser.SelectByIndex]
// @param text - the visible option text to match
//
// @example
//
//	_, err := browser.SelectByText(ctx, browserscale.CSS("select#country"), "Germany")
func (c *CloudBrowser) SelectByText(ctx context.Context, target *Locator, text string) (*SelectOptionResult, error) {
	return c.selectByTextWith(ctx, target, text, SelectOpts{})
}

// SelectByTextWith is the customizable variant of [CloudBrowser.SelectByText].
//
// @inheritDoc [CloudBrowser.SelectByText]
// @param opts - select customization; see [SelectOpts]
//
// @example
//
//	_, err := browser.SelectByTextWith(ctx, browserscale.CSS("select#country"), "Germany", browserscale.SelectOpts{
//	    NoEvents: true,
//	})
func (c *CloudBrowser) SelectByTextWith(ctx context.Context, target *Locator, text string, opts SelectOpts) (*SelectOptionResult, error) {
	return c.selectByTextWith(ctx, target, text, opts)
}

func (c *CloudBrowser) selectByTextWith(ctx context.Context, target *Locator, text string, o SelectOpts) (*SelectOptionResult, error) {
	t := text
	return c.runSelect(ctx, target, o, func(req *generated.SelectOptionRequest) { req.Text = &t })
}

func (c *CloudBrowser) runSelect(ctx context.Context, target *Locator, o SelectOpts, withKey func(*generated.SelectOptionRequest)) (*SelectOptionResult, error) {
	if err := target.validateTarget("SelectOption", false); err != nil {
		return nil, err
	}
	req := &generated.SelectOptionRequest{
		SessionId:     c.sessionId,
		ApiKey:        c.apiKey,
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
	resp, err := c.client.SelectOption(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SelectOptionResult{
		SelectedIndex: resp.SelectedIndex,
		SelectedValue: resp.SelectedValue,
		SelectedText:  resp.SelectedText,
	}, nil
}
