package browserscale

import (
	"context"
	"errors"

	"github.com/browserscale/browserscale-go/generated"
)

const codeStaleRef = "stale_ref"

// StaleRefError is returned when a call was given a reference whose element
// is gone: the document it was seen in was replaced by a navigation or a
// reload, or the element was removed from it. A reference is never
// re-interpreted, so the call fails instead of acting on whatever took the
// element's place. Look again ([Page.Wait], [Page.GetObservation]) and go on
// with the new reference.
//
// It wraps the call's own error ([ClickError], [FillError], [CommandError],
// ...), which errors.As still finds and whose Code is "stale_ref".
//
// @example
//
//	res, _ := page.Wait(ctx, browserscale.CSS(".result"))
//	_, err := page.Click(ctx, res.Element.Ref)
//	var stale *browserscale.StaleRefError
//	if errors.As(err, &stale) {
//	    log.Printf("the page moved on to %s", stale.URL)
//	    res, _ = page.Wait(ctx, browserscale.CSS(".result"))
//	}
type StaleRefError struct {
	// Ref is the reference that went stale: the call's target, or the
	// container it was to be looked for in.
	Ref *Locator
	// URL is where the frame the element was seen in is now, or the page's
	// URL when that frame is gone too. When it differs from the URL the
	// element was found at, the frame navigated; when it is the same, the
	// document was reloaded or the element removed. Empty if it could not be
	// read.
	URL string
	err error
}

// Error implements the error interface.
func (e *StaleRefError) Error() string { return e.err.Error() }

// Unwrap returns the call's own error.
func (e *StaleRefError) Unwrap() error { return e.err }

// checked is the last step of every call that takes a target: it turns the
// two failures that are about the page rather than the element into their
// own error types.
func (p *Page) checked(ctx context.Context, target *Locator, err error) error {
	if err == nil {
		return nil
	}
	err = p.notFocused(err)
	if errorCode(err) != codeStaleRef {
		return err
	}
	ref := target.firstRef()
	return &StaleRefError{Ref: ref, URL: p.currentURL(ctx, ref), err: err}
}

// firstRef is the reference the locator is, or the first one among the
// containers it is looked for in.
func (l *Locator) firstRef() *Locator {
	for cur := l; cur != nil; cur = cur.in {
		if cur.ref != "" {
			return cur
		}
	}
	return nil
}

// currentURL reads where the frame a reference was seen in is now, falling
// back to the page's own URL. Best effort: "" when the page cannot be read.
func (p *Page) currentURL(ctx context.Context, ref *Locator) string {
	resp, err := p.client.GetPages(ctx, &generated.GetPagesRequest{
		SessionId: p.sessionId, ApiKey: p.apiKey,
	})
	if err != nil {
		return ""
	}
	for _, pg := range resp.GetPages() {
		if pg.GetPageId() != p.id {
			continue
		}
		if ref != nil && ref.seenIn != "" {
			if f := findFrame(pg.GetFrameTree(), ref.seenIn); f != nil {
				return f.GetUrl()
			}
		}
		return pg.GetUrl()
	}
	return ""
}

func findFrame(f *generated.FrameInfo, frameID string) *generated.FrameInfo {
	if f == nil {
		return nil
	}
	if f.GetFrameId() == frameID {
		return f
	}
	for _, c := range f.GetChildren() {
		if hit := findFrame(c, frameID); hit != nil {
			return hit
		}
	}
	return nil
}

// errorCode is the failure code of any of the SDK's typed errors, or "".
func errorCode(err error) string {
	var (
		click  *ClickError
		fill   *FillError
		drag   *DragError
		sel    *SelectOptionError
		scroll *ScrollError
		move   *MoveError
		cmd    *CommandError
	)
	switch {
	case errors.As(err, &click):
		return click.Code
	case errors.As(err, &fill):
		return fill.Code
	case errors.As(err, &drag):
		return drag.Code
	case errors.As(err, &sel):
		return sel.Code
	case errors.As(err, &scroll):
		return scroll.Code
	case errors.As(err, &move):
		return move.Code
	case errors.As(err, &cmd):
		return cmd.Code
	}
	return ""
}
