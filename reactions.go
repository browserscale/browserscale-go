package browserscale

import (
	"context"
	"fmt"

	"github.com/browserscale/browserscale-go/generated"
)

// ReactionInfo describes a still-pending reaction, as returned by
// [Page.ListReactions] and [CloudBrowser.ListReactions]. One-shot reactions
// that have already fired are gone and never appear here.
type ReactionInfo struct {
	// ReactionID is the stable id assigned by AddReaction (pass to RemoveReaction).
	ReactionID string
	// MatchSelector is set if the reaction matches by CSS selector.
	MatchSelector string
	// MatchJsExpression is set if the reaction matches by JS expression.
	MatchJsExpression string
	// ActionSelector is set if the click target differs from the matched element.
	ActionSelector string
	// ActionJsExpression is set if the click target differs from the matched element.
	ActionJsExpression string
	// FrameID is the frame scope: "" for the main frame, a specific frameId, or
	// [AllFrames].
	FrameID string
	// Visible reports whether the match additionally requires visibility.
	Visible bool
	// AllPages reports whether the reaction was registered for every page
	// ([CloudBrowser.AddReaction]) rather than for one.
	AllPages bool
}

// ReactionOpts customizes [Page.AddReactionWith] and
// [CloudBrowser.AddReactionWith].
// Zero/empty values mean "use the server default".
type ReactionOpts struct {
	// On overrides the click target. Nil = click the matched element itself.
	// Provide a CSS or JS [Locator] to click a different element, resolved in
	// the matched element's frame (e.g. a modal's close "X"). Node/At locators
	// are rejected.
	On *Locator

	// Button is the mouse button for the click.
	// Valid: "left" (default), "right", "middle".
	Button string

	// ClickCount controls single/double-click.
	// 0 or 1 = single click (default), 2 = double-click.
	ClickCount int32

	// IntervalMs is the poll cadence in milliseconds for the shared page loop.
	// 0 = server default (300ms).
	IntervalMs float64
}

// AddReaction registers a one-shot "reaction": the browser watches for the match
// locator in the background and, as soon as it matches, clicks it the same way
// [Page.Click] does (scroll, human path, occlusion check) — then removes
// itself. When it fires, the input action you are running pauses at a safe
// point (a click before it presses, a fill between two keys), the reaction
// clicks, and the action carries on within its own timeout — e.g. it
// dismisses a newsletter modal blocking a [Page.Click], and the click then
// lands. A captcha solve takes the page first, a button held down by a
// press-only click keeps reactions out until its release, and a reaction
// waits for its page to have the focus. A reaction added here belongs to this
// page and is torn down automatically when the page/session ends; use
// [CloudBrowser.AddReaction] for one that covers every page. Each click is
// reported as a [ContextEventReactionFired] on [CloudBrowser.ContextEvents].
//
// match must be a CSS or JS [Locator] — [Node] and [At] are rejected. Use
// [Locator.InAllFrames] to watch every frame and [Locator.Visible](false) to
// opt out of the default visibility gate.
//
// @param match - the CSS/JS locator to watch for
//
// @returns the reactionId (pass to [Page.RemoveReaction])
//
// A match that is nil, carries neither a selector nor a JS expression, or is a
// Node/At locator is rejected before anything is sent. Beyond that, reports only
// transport failures: registering a reaction has no semantic failure of its own,
// and says nothing about whether the element it watches ever appears.
//
// @see [Page.AddReactionWith] for a different click target, button,
//
//	double-click or poll cadence
//
// @example
//
//	// Auto-dismiss a consent button whenever it appears, in any frame.
//	id, err := page.AddReaction(ctx, browserscale.CSS("button#accept").InAllFrames())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_ = id
func (p *Page) AddReaction(ctx context.Context, match *Locator) (string, error) {
	return addReaction(ctx, p.client, p.sessionId, p.apiKey, p.id, match, ReactionOpts{})
}

// AddReactionWith is the customizable variant of [Page.AddReaction].
//
// @inheritDoc [Page.AddReaction]
// @param opts - reaction customization; see [ReactionOpts]
//
// @example
//
//	// Watch for a newsletter modal, but click its close "X" instead.
//	id, err := page.AddReactionWith(ctx,
//	    browserscale.CSS("#newsletter-modal"),
//	    browserscale.ReactionOpts{On: browserscale.CSS(".modal-close")},
//	)
func (p *Page) AddReactionWith(ctx context.Context, match *Locator, opts ReactionOpts) (string, error) {
	return addReaction(ctx, p.client, p.sessionId, p.apiKey, p.id, match, opts)
}

// AddReaction registers a one-shot reaction for every page of the browser:
// it is armed on the pages open now and on every page that opens later —
// tabs, popups, pages you create — and fires once on each. Use it for the
// interruptions a site shows wherever you are on it, such as a consent
// banner. Otherwise it behaves exactly like [Page.AddReaction].
//
// The reaction stays registered until you remove it or the session ends;
// [CloudBrowser.ListReactions] lists it, and [Page.ListReactions] lists it on
// each page where it has not fired yet. Every click is reported as a
// [ContextEventReactionFired], carrying the page it fired on.
//
// @param match - the CSS/JS locator to watch for
//
// @returns the reactionId (pass to [CloudBrowser.RemoveReaction])
//
// A match that is nil, carries neither a selector nor a JS expression, or is a
// Node/At locator is rejected before anything is sent. Beyond that, reports only
// transport failures.
//
// @see [CloudBrowser.AddReactionWith] for a different click target, button,
//
//	double-click or poll cadence
//
// @example
//
//	// Accept the consent banner on whichever page it shows up.
//	id, err := browser.AddReaction(ctx, browserscale.CSS("button#accept").InAllFrames())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_ = id
func (c *CloudBrowser) AddReaction(ctx context.Context, match *Locator) (string, error) {
	return addReaction(ctx, c.client, c.sessionId, c.apiKey, "", match, ReactionOpts{})
}

// AddReactionWith is the customizable variant of [CloudBrowser.AddReaction].
//
// @inheritDoc [CloudBrowser.AddReaction]
// @param opts - reaction customization; see [ReactionOpts]
//
// @example
//
//	// Close the newsletter modal on every page that opens it.
//	id, err := browser.AddReactionWith(ctx,
//	    browserscale.CSS("#newsletter-modal"),
//	    browserscale.ReactionOpts{On: browserscale.CSS(".modal-close")},
//	)
func (c *CloudBrowser) AddReactionWith(ctx context.Context, match *Locator, opts ReactionOpts) (string, error) {
	return addReaction(ctx, c.client, c.sessionId, c.apiKey, "", match, opts)
}

// addReaction registers a reaction on pageID, or on every page if it is "".
func addReaction(ctx context.Context, client generated.BrowserClient, sessionID, apiKey, pageID string, match *Locator, o ReactionOpts) (string, error) {
	if err := validateReactionMatch("AddReaction", match); err != nil {
		return "", err
	}

	req := &generated.AddReactionRequest{
		SessionId:         sessionID,
		PageId:            pageID,
		ApiKey:            apiKey,
		MatchSelector:     strPtr(match.selector),
		MatchJsExpression: strPtr(match.jsExpression),
		FrameId:           pickFrame("", match),
		Visible:           match.visible,
		Button:            strPtr(o.Button),
		ClickCount:        intPtr(o.ClickCount),
		Interval:          floatPtrIfNonZero(o.IntervalMs),
	}
	if o.On != nil {
		if err := validateReactionAction("AddReaction", o.On); err != nil {
			return "", err
		}
		req.ActionSelector = strPtr(o.On.selector)
		req.ActionJsExpression = strPtr(o.On.jsExpression)
	}

	resp, err := client.AddReaction(ctx, req)
	if err != nil {
		return "", err
	}
	if e := commandErrorFrom("addReaction", resp.GetError()); e != nil {
		return "", e
	}
	return resp.ReactionId, nil
}

// RemoveReaction removes a pending reaction by id. It returns false if the
// reaction had already fired (one-shot) or was never registered. An id from
// [CloudBrowser.AddReaction] is removed from every page; it is the same call
// as [CloudBrowser.RemoveReaction].
//
// @param reactionID - id returned by [Page.AddReaction]
//
// @returns true if a pending reaction with this id existed and was removed
//
// Removing an id that is not registered is a no-op rather than an error, so the
// returned bool - not the error - is what tells you whether anything was there.
// Reports only transport failures.
//
// @example
//
//	removed, err := page.RemoveReaction(ctx, id)
func (p *Page) RemoveReaction(ctx context.Context, reactionID string) (bool, error) {
	return removeReaction(ctx, p.client, p.sessionId, p.apiKey, reactionID)
}

// RemoveReaction removes a reaction by id, wherever it is pending: a
// reaction from [CloudBrowser.AddReaction] stops for every page, including
// the ones that open later. It returns false if no reaction with this id was
// pending anywhere.
//
// @param reactionID - id returned by [CloudBrowser.AddReaction] or [Page.AddReaction]
//
// @returns true if a pending reaction with this id existed and was removed
//
// Removing an id that is not registered is a no-op rather than an error.
// Reports only transport failures.
//
// @example
//
//	removed, err := browser.RemoveReaction(ctx, id)
func (c *CloudBrowser) RemoveReaction(ctx context.Context, reactionID string) (bool, error) {
	return removeReaction(ctx, c.client, c.sessionId, c.apiKey, reactionID)
}

func removeReaction(ctx context.Context, client generated.BrowserClient, sessionID, apiKey, reactionID string) (bool, error) {
	resp, err := client.RemoveReaction(ctx, &generated.RemoveReactionRequest{
		SessionId:  sessionID,
		ApiKey:     apiKey,
		ReactionId: reactionID,
	})
	if err != nil {
		return false, err
	}
	if e := commandErrorFrom("removeReaction", resp.GetError()); e != nil {
		return false, e
	}
	return resp.Removed, nil
}

// ListReactions returns the reactions still pending on the current page,
// including those from [CloudBrowser.AddReaction] that have not fired here
// yet ([ReactionInfo.AllPages] is set on those). Reactions that have already
// fired on this page are not included.
//
// @returns the pending reactions for the page
//
// Reports only transport failures - a dead session, a page that is gone, a broken
// connection. This call has no semantic failure of its own, so there are no error
// codes to branch on.
//
// @example
//
//	pending, err := page.ListReactions(ctx)
//	for _, r := range pending {
//	    log.Printf("reaction %s watching %s%s", r.ReactionID, r.MatchSelector, r.MatchJsExpression)
//	}
func (p *Page) ListReactions(ctx context.Context) ([]ReactionInfo, error) {
	return listReactions(ctx, p.client, p.sessionId, p.apiKey, p.id)
}

// ListReactions returns the reactions registered with
// [CloudBrowser.AddReaction] that have not been removed. Each stays listed
// while it fires on page after page; [Page.ListReactions] shows where it is
// still pending.
//
// @returns the reactions registered for every page
//
// Reports only transport failures. This call has no semantic failure of its
// own, so there are no error codes to branch on.
//
// @example
//
//	all, err := browser.ListReactions(ctx)
//	for _, r := range all {
//	    log.Printf("browser-wide reaction %s", r.ReactionID)
//	}
func (c *CloudBrowser) ListReactions(ctx context.Context) ([]ReactionInfo, error) {
	return listReactions(ctx, c.client, c.sessionId, c.apiKey, "")
}

// listReactions lists pageID's pending reactions, or the browser-wide ones if
// it is "".
func listReactions(ctx context.Context, client generated.BrowserClient, sessionID, apiKey, pageID string) ([]ReactionInfo, error) {
	resp, err := client.ListReactions(ctx, &generated.ListReactionsRequest{
		SessionId: sessionID,
		PageId:    pageID,
		ApiKey:    apiKey,
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("listReactions", resp.GetError()); e != nil {
		return nil, e
	}
	out := make([]ReactionInfo, 0, len(resp.GetReactions()))
	for _, r := range resp.GetReactions() {
		out = append(out, ReactionInfo{
			ReactionID:         r.GetReactionId(),
			MatchSelector:      r.GetMatchSelector(),
			MatchJsExpression:  r.GetMatchJsExpression(),
			ActionSelector:     r.GetActionSelector(),
			ActionJsExpression: r.GetActionJsExpression(),
			FrameID:            r.GetFrameId(),
			Visible:            r.GetVisible(),
			AllPages:           r.GetAllPages(),
		})
	}
	return out, nil
}

// validateReactionMatch enforces that the match is a CSS/JS locator (like a
// Wait condition): a selector or JS expression is required, and Node/At are
// rejected.
func validateReactionMatch(cmd string, l *Locator) error {
	if l == nil {
		return fmt.Errorf("browserscale.%s: a match locator is required", cmd)
	}
	if l.backendNodeId != 0 || l.x != nil || l.y != nil {
		return fmt.Errorf("browserscale.%s: match must be a CSS/JS locator (Node()/At() are not valid reaction matches)", cmd)
	}
	if l.selector == "" && l.jsExpression == "" {
		return fmt.Errorf("browserscale.%s: match must have a CSS selector or JS expression", cmd)
	}
	return nil
}

// validateReactionAction enforces that the optional click target (ReactionOpts.On)
// is a CSS/JS locator resolved in the matched element's frame — Node/At and
// per-target frame overrides are not meaningful here.
func validateReactionAction(cmd string, l *Locator) error {
	if l.backendNodeId != 0 || l.x != nil || l.y != nil {
		return fmt.Errorf("browserscale.%s: On must be a CSS/JS locator (Node()/At() are not valid action targets)", cmd)
	}
	if l.selector == "" && l.jsExpression == "" {
		return fmt.Errorf("browserscale.%s: On must have a CSS selector or JS expression", cmd)
	}
	return nil
}
