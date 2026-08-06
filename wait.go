package browserscale

import (
	"context"
	"errors"
	"fmt"

	"github.com/browserscale/browserscale-go/generated"
)

// WaitArg is the marker interface for everything Wait accepts: a Locator
// (treated as a condition) or a wait-level option such as Timeout / InFrame
// / InAllFrames.
type WaitArg interface{ applyWait(*waitCall) }

type waitCall struct {
	conditions []*Locator
	timeout    float64
	frameId    string
}

// Locator implements WaitArg by appending itself to the call's condition
// list.
func (l *Locator) applyWait(w *waitCall) {
	w.conditions = append(w.conditions, l)
}

// ── Wait-only options ──

type waitOpt func(*waitCall)

func (f waitOpt) applyWait(w *waitCall) { f(w) }

// Timeout overrides the [CloudBrowser.Wait] timeout.
//
// When omitted, [DefaultWaitTimeoutMs] (30s) is used. Pass once per Wait
// call as one of the variadic arguments.
//
// @param ms - timeout in milliseconds
//
// @returns a [WaitArg] suitable for passing to Wait
//
// @example
//
//	_, _ = browser.Wait(ctx, browserscale.CSS("#done"), browserscale.Timeout(5000))
func Timeout(ms float64) WaitArg { return waitOpt(func(w *waitCall) { w.timeout = ms }) }

// ── Wait ──

// Wait blocks until any of the supplied locators matches.
//
// Pass one or more [Locator]s (built with [CSS], [JS], …) plus optional
// wait-level arguments such as [Timeout]. When several locators are
// supplied, the first one to match wins; the others are abandoned.
//
// Defaults applied automatically:
//   - timeout: [DefaultWaitTimeoutMs] (30s) — override with [Timeout]
//   - per-locator visible/steady: [DefaultVisible] (true) and
//     [DefaultSteadyMs] (500) for CSS and JS locators. For JS expressions
//     returning a non-Element value (bool/string/number/object) both
//     flags are no-ops. Override with [Locator.Visible] / [Locator.Steady]
//     on individual locators.
//
// [Node] and [At] are not valid wait conditions — they only make sense as
// action targets — and produce an error at send time.
//
// @param args - one or more [Locator]s plus optional wait-level options;
//
//	at least one [Locator] is required
//
// @returns *WaitResult for the first matching condition (carries the
//
//	matched condition's index, frameId, backendNodeId and bounds)
//
// If no condition matched before the timeout, err is a [*WaitError] carrying
// the failure code and a per-condition breakdown (Conditions) of why each
// element never matched — e.g. "found_occluded" with the intercepting element.
// The *WaitResult is still returned (with Index -1). Recover the detail with
// errors.As. A malformed call (no conditions, or a condition without a
// selector/JS expression) returns an ordinary error instead.
//
// @see [WaitError] for the timeout detail
//
// @example
//
//	// Wait for either a success banner or a JS condition, max 5s.
//	res, err := browser.Wait(ctx,
//	    browserscale.CSS(".success"),
//	    browserscale.JS("window.__ready === true"),
//	    browserscale.Timeout(5000),
//	)
//	if err != nil {
//	    var we *browserscale.WaitError
//	    if errors.As(err, &we) {
//	        for _, c := range we.Conditions {
//	            log.Printf("condition %d: %s", c.Index, c.State)
//	        }
//	    }
//	    log.Fatal(err)
//	}
//	_ = res
func (c *CloudBrowser) Wait(ctx context.Context, args ...WaitArg) (*WaitResult, error) {
	wc := &waitCall{timeout: DefaultWaitTimeoutMs}
	for _, a := range args {
		a.applyWait(wc)
	}
	if len(wc.conditions) == 0 {
		return nil, errors.New("browserscale.Wait: at least one condition required")
	}

	pbConds := make([]*generated.WaitCondition, len(wc.conditions))
	for i, cond := range wc.conditions {
		if cond.selector == "" && cond.jsExpression == "" {
			return nil, fmt.Errorf("browserscale.Wait: condition %d has neither selector nor JS expression (Node()/At() are not valid wait conditions)", i)
		}
		pc := &generated.WaitCondition{}
		if cond.selector != "" {
			s := cond.selector
			pc.Selector = &s
		}
		if cond.jsExpression != "" {
			e := cond.jsExpression
			pc.JsExpression = &e
		}
		if cond.visible != nil {
			v := *cond.visible
			pc.Visible = &v
		}
		if cond.steadyTime != nil {
			st := *cond.steadyTime
			pc.SteadyTime = &st
		}
		// If no call-level frameId was set, fall back to the first cond's frameId.
		if wc.frameId == "" && cond.frameId != "" {
			wc.frameId = cond.frameId
		}
		pbConds[i] = pc
	}

	req := &generated.WaitForAnyParams{
		SessionId:  c.sessionId,
		ApiKey:     c.apiKey,
		Conditions: pbConds,
		Timeout:    floatPtrIfNonZero(wc.timeout),
	}
	if wc.frameId != "" {
		fid := wc.frameId
		req.FrameId = &fid
	}

	resp, err := c.client.WaitForAny(ctx, req)
	if err != nil {
		return nil, err
	}
	// A timeout (no condition matched) comes back as index=-1 with a structured
	// detail rather than a gRPC error. Surface it through err as a *WaitError so
	// the res, err shape stays identical to the other calls.
	res, waitErr := waitResultFromProto(resp)
	if waitErr != nil {
		return res, waitErr
	}
	return res, nil
}
