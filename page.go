package browserscale

import (
	"context"
	"errors"
	"strings"

	"github.com/browserscale/browserscale-go/generated"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Page is one page (tab or popup) of the browser. Everything that acts on a
// document — navigating, waiting, clicking, typing, reading — is a Page method.
//
// A Page is only the page's id: it holds no state and asks for none, so it
// never goes stale on its own. Which pages exist and which has the focus is
// the browser's to know. When the page closes, its next call fails with a
// [PageClosedError] that names the page that has the focus now.
//
// A browser starts without a page: [CloudBrowser.CreatePage] opens the first.
// Get one from [CloudBrowser.CreatePage], [CloudBrowser.FocusedPage],
// [CloudBrowser.Pages] or [CloudBrowser.GetPage].
type Page struct {
	browser   *CloudBrowser
	client    generated.BrowserClient
	sessionId string
	apiKey    string
	id        string
}

// Id is the page's id, stable for the life of the page.
func (p *Page) Id() string { return p.id }

// Browser is the browser the page belongs to.
func (p *Page) Browser() *CloudBrowser { return p.browser }

// GetPage returns the [Page] with this id. It makes no call and does not check
// the id: a page that is gone shows on the Page's first call.
//
// @param pageId - a page id, as reported by [CloudBrowser.Pages] or an error
//
// @example
//
//	page := browser.GetPage(savedPageId)
//	_, err := page.Navigate(ctx, "https://example.com", 0)
func (c *CloudBrowser) GetPage(pageId string) *Page {
	return &Page{browser: c, client: c.client, sessionId: c.sessionId, apiKey: c.apiKey, id: pageId}
}

// CreatePage opens a page the way opening a new tab does: an about:blank tab
// that takes the focus. A browser starts without a page, so this is the first
// call after renting one. Returns once the page is ready for [Page.Navigate].
//
// @throws create_failed - the browser could not open the page, typically
// capacity
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	page, err := browser.CreatePage(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_, err = page.Navigate(ctx, "https://example.com", 0)
func (c *CloudBrowser) CreatePage(ctx context.Context) (*Page, error) {
	resp, err := c.client.CreatePage(ctx, &generated.CreatePageRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("createPage", resp.GetError()); e != nil {
		return nil, e
	}
	return c.GetPage(resp.GetPage().GetPageId()), nil
}

// FocusedPage returns the page that has the focus: the one a person would be
// looking at. Once the browser has a page, exactly one has it. A tab or popup
// a page opens takes it, and closing the focused page hands it on, the way it
// does for a person.
//
// Input ([Page.Click], [Page.MoveTo] and what builds on them) only acts on
// the focused page; [Page.Focus] moves the focus.
//
// @throws [ErrNoPage] - the browser has no page yet; [CloudBrowser.CreatePage]
// opens the first
//
// @example
//
//	page, err := browser.FocusedPage(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_, err = page.Navigate(ctx, "https://example.com", 0)
func (c *CloudBrowser) FocusedPage(ctx context.Context) (*Page, error) {
	resp, err := c.client.GetFocusedPage(ctx, &generated.GetFocusedPageRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	return c.GetPage(resp.PageId), nil
}

// Focus brings the page to the front, the way clicking its tab does: it
// becomes visible, and the page it replaces is hidden. Focusing the page that
// already has the focus changes nothing.
//
// @throws [PageClosedError] - the page has closed
//
// @example
//
//	if err := page.Focus(ctx); err != nil {
//	    log.Fatal(err)
//	}
func (p *Page) Focus(ctx context.Context) error {
	resp, err := p.client.FocusPage(ctx, &generated.FocusPageRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("focusPage", resp.GetError())
}

// Close closes the page the way closing its tab does. Its unload handlers
// run, but no "leave site?" prompt is shown. Returns once the page is gone.
//
// @returns *Page the page that has the focus afterwards; when the closed page
//
//	had it, the browser hands it on, usually to the page that opened it
//
// @throws last_page - it is the browser's only page; a browser keeps at least
// one, stop the browser to end it
// @throws timeout - the close was started but the page was still open after a
// few seconds
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	page, err = popup.Close(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (p *Page) Close(ctx context.Context) (*Page, error) {
	resp, err := p.client.ClosePage(ctx, &generated.ClosePageRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("closePage", resp.GetError()); e != nil {
		return nil, e
	}
	return p.browser.GetPage(resp.GetFocusedPageId()), nil
}

// PageClosedError is returned by a [Page] call when the page has closed.
// FocusedPage is the page that has the focus now, usually the one that opened
// the closed page.
//
// @example
//
//	var closed *browserscale.PageClosedError
//	if errors.As(err, &closed) {
//	    page = closed.FocusedPage
//	}
type PageClosedError struct {
	// PageId is the id of the page that closed.
	PageId string
	// FocusedPage is the page that has the focus now.
	FocusedPage *Page
}

// Error implements the error interface.
func (e *PageClosedError) Error() string {
	return "page " + e.PageId + " was closed; page " + e.FocusedPage.Id() + " has the focus"
}

// PageNotFocusedError is returned by [Page.Click], [Page.MoveTo] and the
// calls built on them ([Page.Fill], [Page.DragBy], [Page.DragTo]) when the
// page does not have the focus, typically because it opened a tab or popup
// that came to the front. FocusedPage is that page; [Page.Focus] brings this
// one back.
//
// It wraps the call's own error ([ClickError], [MoveError], [FillError],
// [DragError]), which errors.As still finds.
//
// @example
//
//	var nf *browserscale.PageNotFocusedError
//	if errors.As(err, &nf) {
//	    page = nf.FocusedPage // continue in the new tab
//	}
type PageNotFocusedError struct {
	// FocusedPage is the page that has the focus.
	FocusedPage *Page
	err         error
}

// Error implements the error interface.
func (e *PageNotFocusedError) Error() string { return e.err.Error() }

// Unwrap returns the call's own error.
func (e *PageNotFocusedError) Unwrap() error { return e.err }

const codePageNotFocused = "page_not_focused"

// notFocused wraps err in a *PageNotFocusedError when it says the page does
// not have the focus, and returns it unchanged otherwise.
func (p *Page) notFocused(err error) error {
	focused, ok := focusedPageIdOf(err)
	if !ok {
		return err
	}
	return &PageNotFocusedError{FocusedPage: p.browser.GetPage(focused), err: err}
}

func focusedPageIdOf(err error) (string, bool) {
	var ce *ClickError
	if errors.As(err, &ce) && ce.Code == codePageNotFocused {
		return ce.FocusedPageId, true
	}
	var me *MoveError
	if errors.As(err, &me) && me.Code == codePageNotFocused {
		return me.FocusedPageId, true
	}
	var fe *FillError
	if errors.As(err, &fe) && fe.ClickError != nil && fe.ClickError.Code == codePageNotFocused {
		return fe.ClickError.FocusedPageId, true
	}
	var de *DragError
	if errors.As(err, &de) && de.ClickError != nil && de.ClickError.Code == codePageNotFocused {
		return de.ClickError.FocusedPageId, true
	}
	return "", false
}

// ErrNoPage is returned by the calls that need the focused page
// ([CloudBrowser.FocusedPage], [CloudBrowser.StartStream]) while the browser
// has no page yet. A browser starts without one; [CloudBrowser.CreatePage]
// opens the first.
//
// @example
//
//	if errors.Is(err, browserscale.ErrNoPage) {
//	    page, err = browser.CreatePage(ctx)
//	}
var ErrNoPage = errors.New("the browser has no page yet; CreatePage opens one")

const (
	pageClosedPrefix = "PAGE_CLOSED: focusedPageId="
	noPagePrefix     = "NO_PAGE:"
)

// pageClosedInterceptor turns the server's PAGE_CLOSED status into a
// *PageClosedError and its NO_PAGE status into [ErrNoPage]. Installed on the
// connection, so every call gets it.
func (c *CloudBrowser) pageClosedInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		return err
	}
	if strings.HasPrefix(st.Message(), noPagePrefix) {
		return ErrNoPage
	}
	focused, ok := strings.CutPrefix(st.Message(), pageClosedPrefix)
	if !ok {
		return err
	}
	closed := ""
	if r, ok := req.(interface{ GetPageId() string }); ok {
		closed = r.GetPageId()
	}
	return &PageClosedError{PageId: closed, FocusedPage: c.GetPage(focused)}
}
