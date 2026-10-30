package browserscale

import (
	"context"
	"fmt"

	"github.com/browserscale/browserscale-go/generated"
)

// SolveCaptcha detects and solves the first supported bot-challenge it
// finds anywhere on the page.
//
// Detection covers the common challenge types you run into in the wild.
// The challenge is detected and solved in the live browser session — no page
// scripting, no external service — and the result is wired back into the page
// automatically, so callers can ignore the returned string.
//
// One captcha is solved at a time per page: if auto-solve
// ([CloudBrowser.EnableCaptchaAutoSolve]) is already solving one there, the
// call waits for that solve and returns its outcome instead of starting a
// second one. Like an auto-solve, the solve holds the actions running on the
// page and is reported on [CloudBrowser.ContextEvents] (Trigger "command").
//
// @param timeoutMs - how long to wait for a captcha to appear, in
//
//	milliseconds; 0 uses the server default (60s)
//
// @param retryAmount - number of retries on a failed solve before giving up
//
// @returns empty string on success — the solution is applied server-side
//
// A challenge that was not solved is reported as a [*SolveCaptchaError] with a
// code and, when one was found, its Kind:
//
// @throws not_found - no challenge waiting for an answer appeared within
// timeoutMs. One that passed on its own does not count, so the page may simply
// be through
// @throws unsupported - a challenge was found that cannot be solved yet
// @throws solve_failed - the solver answered it and it did not clear within
// retryAmount rounds
// @throws blocked - it was answered and the site replied with a block instead
// of letting the visitor through; solving again in this session will not help
// @throws timeout - timeoutMs ran out while the solver was at it
// @throws page_closed - the page closed during the solve
// @throws cancelled - a navigation of the page started during the solve
//
// @see [SolveCaptchaError] for recovering the code and kind with errors.As
//
// @example
//
//	if _, err := page.SolveCaptcha(ctx, 0, 2); err != nil {
//	    var ce *browserscale.SolveCaptchaError
//	    if errors.As(err, &ce) && ce.Code == "blocked" {
//	        // start over in a fresh session
//	    }
//	    log.Fatal(err)
//	}
func (p *Page) SolveCaptcha(ctx context.Context, timeoutMs, retryAmount int32) (string, error) {
	resp, err := p.client.SolveCaptcha(ctx, &generated.SolveCaptchaRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		TimeoutMs:     timeoutMs,
		RetryAmount:   retryAmount,
		ReportFailure: true,
	})
	if err != nil {
		return "", err
	}
	if e := resp.GetError(); e != nil {
		return "", &SolveCaptchaError{Code: e.Code, Message: e.Message, Kind: resp.Kind, SolveId: resp.GetSolveId()}
	}
	return resp.Result, nil
}

// CaptchaAutoSolveOpts configures [CloudBrowser.EnableCaptchaAutoSolve].
// Zero values use the server defaults.
type CaptchaAutoSolveOpts struct {
	// SolveTimeoutMs is the budget for one solve, in milliseconds. 0 = 60000.
	SolveTimeoutMs int32

	// RetryAmount is the per-solve retry cap, as in [Page.SolveCaptcha].
	// 0 = the solver's own default.
	RetryAmount int32

	// OnFailure is what an action held by a solve does when the solve fails.
	// "" or "fail" = it ends with code "captcha_failed" (default).
	// "continue" = it carries on as if the solve had succeeded.
	OnFailure string

	// Unfocused is what happens to a captcha on a page that does not have the
	// focus. Input only reaches the focused page, so it cannot be solved there
	// as it is.
	// "" or "wait" = it is reported (State "waiting_for_focus") and solved
	// once the page has the focus (default).
	// "focus" = the page is brought to the front and it is solved right away.
	Unfocused string

	// MaxSolvesPerMinute caps the solves that start per page within any 60
	// seconds. Further captchas are reported as "skipped" and resolved with
	// code "limit_reached". Protects against a page that keeps putting one
	// up. 0 = 5.
	MaxSolvesPerMinute int32
}

// EnableCaptchaAutoSolve makes the browser solve captchas as they appear, on
// every page, frame and popup of the session, including ones opened later.
// Nothing has to call [Page.SolveCaptcha] or wait for a [Captcha] condition.
//
// An action running on the page when a captcha appears ([Page.Click],
// [Page.Fill], [Page.Type], [Page.DragBy], [Page.DragTo]) is held at its next
// safe point — between two keys, before the button goes down — and carries on
// after the solve: a fill types the rest of its text, a click goes for its
// target again. Its result lists the solve under Captchas. An action started
// while a solve runs waits for it. Time spent held does not count against the
// action's timeout, nor against a [Page.Wait]. [Page.InsertText],
// [Page.PressKey] and [Page.ReleaseKey] are refused with code "busy" while a
// solve runs.
//
// While a [Page.Wait] with a [Captcha] condition runs on a page, auto-solve
// leaves that page's captchas alone, so the wait sees them first.
//
// Each solve is reported on [CloudBrowser.ContextEvents]: a
// [ContextEventCaptchaDetected] and then exactly one
// [ContextEventCaptchaResolved] with the same SolveId.
//
// Calling it again replaces the settings; solves already running keep the
// ones they started with. Auto-solve lasts until
// [CloudBrowser.DisableCaptchaAutoSolve] or the end of the session.
//
// @param opts - settings; see [CaptchaAutoSolveOpts]. The zero value uses the
//
//	defaults
//
// An OnFailure or Unfocused value other than the documented ones, or a
// negative number, is rejected before anything is sent. Beyond that, reports
// only transport failures.
//
// @example
//
//	err := browser.EnableCaptchaAutoSolve(ctx, browserscale.CaptchaAutoSolveOpts{
//	    Unfocused: "focus",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// A captcha that shows up during the fill is solved, then the fill
//	// carries on.
//	res, err := page.Fill(ctx, browserscale.CSS("#email"), "user@example.com")
//	if err == nil && len(res.Captchas) > 0 {
//	    log.Printf("held by %d captcha solve(s)", len(res.Captchas))
//	}
func (c *CloudBrowser) EnableCaptchaAutoSolve(ctx context.Context, opts CaptchaAutoSolveOpts) error {
	switch opts.OnFailure {
	case "", "fail", "continue":
	default:
		return fmt.Errorf("browserscale.EnableCaptchaAutoSolve: OnFailure must be \"fail\" or \"continue\", got %q", opts.OnFailure)
	}
	switch opts.Unfocused {
	case "", "wait", "focus":
	default:
		return fmt.Errorf("browserscale.EnableCaptchaAutoSolve: Unfocused must be \"wait\" or \"focus\", got %q", opts.Unfocused)
	}
	if opts.SolveTimeoutMs < 0 || opts.RetryAmount < 0 || opts.MaxSolvesPerMinute < 0 {
		return fmt.Errorf("browserscale.EnableCaptchaAutoSolve: SolveTimeoutMs, RetryAmount and MaxSolvesPerMinute must not be negative")
	}
	req := &generated.EnableCaptchaAutoSolveRequest{SessionId: c.sessionId, ApiKey: c.apiKey}
	if opts.SolveTimeoutMs > 0 {
		req.SolveTimeoutMs = &opts.SolveTimeoutMs
	}
	if opts.RetryAmount > 0 {
		req.RetryAmount = &opts.RetryAmount
	}
	if opts.OnFailure != "" {
		req.OnFailure = &opts.OnFailure
	}
	if opts.Unfocused != "" {
		req.Unfocused = &opts.Unfocused
	}
	if opts.MaxSolvesPerMinute > 0 {
		req.MaxSolvesPerMinute = &opts.MaxSolvesPerMinute
	}
	resp, err := c.client.EnableCaptchaAutoSolve(ctx, req)
	if err != nil {
		return err
	}
	return commandErrorFrom("enableCaptchaAutoSolve", resp.GetError())
}

// DisableCaptchaAutoSolve stops [CloudBrowser.EnableCaptchaAutoSolve]. A solve
// already running finishes; one waiting for the focus is resolved with code
// "disabled".
//
// @returns whether auto-solve was enabled
//
// Reports only transport failures: disabling without auto-solve is a no-op.
//
// @example
//
//	if _, err := browser.DisableCaptchaAutoSolve(ctx); err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) DisableCaptchaAutoSolve(ctx context.Context) (bool, error) {
	resp, err := c.client.DisableCaptchaAutoSolve(ctx, &generated.DisableCaptchaAutoSolveRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return false, err
	}
	if e := commandErrorFrom("disableCaptchaAutoSolve", resp.GetError()); e != nil {
		return false, e
	}
	return resp.GetDisabled(), nil
}
