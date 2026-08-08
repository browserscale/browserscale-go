package browserscale

import (
	"context"
	"fmt"

	"github.com/browserscale/browserscale-go/generated"
)

// ReactionInfo describes a still-pending reaction, as returned by
// [CloudBrowser.ListReactions]. One-shot reactions that have already fired are
// gone and never appear here.
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
}

// ReactionOpts customizes [CloudBrowser.AddReactionWith].
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

// AddReaction registers a one-shot "reaction": a background poller (one shared
// loop per page) watches for the match locator and, as soon as it matches,
// clicks it with the full smart-click machinery (scroll, human path, occlusion
// gate, evade) — then removes itself. The poller yields to any in-flight input
// action and only fires while the pointer is idle, so a reaction naturally
// slots into the gaps of a retrying foreground action (e.g. it dismisses a
// newsletter modal blocking a [CloudBrowser.Click], after which the click's own
// retry succeeds). Reactions are scoped to the page and torn down automatically
// when the page/session ends.
//
// match must be a CSS or JS [Locator] — [Node] and [At] are rejected. Use
// [Locator.InAllFrames] to watch every frame and [Locator.Visible](false) to
// opt out of the default visibility gate.
//
// @param match - the CSS/JS locator to watch for
//
// @returns the reactionId (pass to [CloudBrowser.RemoveReaction])
//
// @throws INVALID_LOCATOR - match is nil, has no selector/JS expression, or is
//
//	a Node/At locator
//
// @see [CloudBrowser.AddReactionWith] for a different click target, button,
//
//	double-click or poll cadence
//
// @example
//
//	// Auto-dismiss a consent button whenever it appears, in any frame.
//	id, err := browser.AddReaction(ctx, browserscale.CSS("button#accept").InAllFrames())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_ = id
func (c *CloudBrowser) AddReaction(ctx context.Context, match *Locator) (string, error) {
	return c.addReactionWith(ctx, match, ReactionOpts{})
}

// AddReactionWith is the customizable variant of [CloudBrowser.AddReaction].
//
// @inheritDoc [CloudBrowser.AddReaction]
// @param opts - reaction customization; see [ReactionOpts]
//
// @example
//
//	// Watch for a newsletter modal, but click its close "X" instead.
//	id, err := browser.AddReactionWith(ctx,
//	    browserscale.CSS("#newsletter-modal"),
//	    browserscale.ReactionOpts{On: browserscale.CSS(".modal-close")},
//	)
func (c *CloudBrowser) AddReactionWith(ctx context.Context, match *Locator, opts ReactionOpts) (string, error) {
	return c.addReactionWith(ctx, match, opts)
}

func (c *CloudBrowser) addReactionWith(ctx context.Context, match *Locator, o ReactionOpts) (string, error) {
	if err := validateReactionMatch("AddReaction", match); err != nil {
		return "", err
	}

	req := &generated.AddReactionRequest{
		SessionId:         c.sessionId,
		ApiKey:            c.apiKey,
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

	resp, err := c.client.AddReaction(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.ReactionId, nil
}

// RemoveReaction removes a pending reaction by id. It returns false if the
// reaction had already fired (one-shot) or was never registered.
//
// @param reactionID - id returned by [CloudBrowser.AddReaction]
//
// @returns true if a pending reaction with this id existed and was removed
//
// @example
//
//	removed, err := browser.RemoveReaction(ctx, id)
func (c *CloudBrowser) RemoveReaction(ctx context.Context, reactionID string) (bool, error) {
	resp, err := c.client.RemoveReaction(ctx, &generated.RemoveReactionRequest{
		SessionId:  c.sessionId,
		ApiKey:     c.apiKey,
		ReactionId: reactionID,
	})
	if err != nil {
		return false, err
	}
	return resp.Removed, nil
}

// ListReactions returns the still-pending reactions registered for the current
// page. Reactions that have already fired (one-shot) are not included.
//
// @returns the pending reactions for the page
//
// @example
//
//	pending, err := browser.ListReactions(ctx)
//	for _, r := range pending {
//	    log.Printf("reaction %s watching %s%s", r.ReactionID, r.MatchSelector, r.MatchJsExpression)
//	}
func (c *CloudBrowser) ListReactions(ctx context.Context) ([]ReactionInfo, error) {
	resp, err := c.client.ListReactions(ctx, &generated.ListReactionsRequest{
		SessionId: c.sessionId,
		ApiKey:    c.apiKey,
	})
	if err != nil {
		return nil, err
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
