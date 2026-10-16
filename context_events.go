package browserscale

import (
	"context"
	"errors"
	"sync"

	"github.com/browserscale/browserscale-go/generated"
)

// ContextEventKind says what a [ContextEvent] reports, and so which of its
// fields are set.
type ContextEventKind int

const (
	// ContextEventSnapshot: Pages holds every page of the browser. It
	// replaces whatever you hold. The first event is always one; another
	// comes only if this reader fell so far behind that events had to be
	// dropped.
	ContextEventSnapshot ContextEventKind = iota
	// ContextEventPageOpened: Opened is the new page, a tab or popup one of
	// the pages opened, or one a call created. A page that takes the focus
	// as it opens is followed by a ContextEventPageFocused.
	ContextEventPageOpened
	// ContextEventPageClosed: Page closed, and FocusedPage has the focus now.
	// A ContextEventPageFocused follows if the focus moved.
	ContextEventPageClosed
	// ContextEventPageFocused: Page has the focus now.
	ContextEventPageFocused
	// ContextEventPageUpdated: Page committed a navigation or set a title;
	// Url and Title are the new values.
	ContextEventPageUpdated
	// ContextEventCaptchaDetected: a captcha was found on Page, by
	// auto-solve ([CloudBrowser.EnableCaptchaAutoSolve]) or by
	// [Page.SolveCaptcha]. Captcha describes it. Exactly one
	// ContextEventCaptchaResolved with the same Captcha.SolveId follows.
	ContextEventCaptchaDetected
	// ContextEventCaptchaResolved: the solve of Captcha.SolveId on Page
	// ended. Captcha describes how.
	ContextEventCaptchaResolved
	// ContextEventReactionFired: a reaction ([Page.AddReaction],
	// [CloudBrowser.AddReaction]) matched on Page and clicked, or tried to.
	// Reaction describes it. A reaction registered for every page reports
	// once per page it fires on.
	ContextEventReactionFired
)

// ContextEvent is one change to the browser's pages, or a captcha solve or a
// reaction's click on one of them. Applying the page events in order to the last snapshot gives the
// pages as they are, without polling [CloudBrowser.Pages]. An event can
// restate what the snapshot before it already shows; each one sets state
// rather than changing it, so applying it again is harmless.
//
// The captcha and reaction events are reports, not state: a listener that fell so far
// behind that it got a new snapshot may have missed some.
type ContextEvent struct {
	Kind ContextEventKind
	// Pages is set for ContextEventSnapshot.
	Pages []*PageInfo
	// Opened is set for ContextEventPageOpened.
	Opened *PageInfo
	// Page is the page the event is about, for every kind but
	// ContextEventSnapshot.
	Page *Page
	// FocusedPage is set for ContextEventPageClosed.
	FocusedPage *Page
	// Url and Title are set for ContextEventPageUpdated.
	Url   string
	Title string
	// Captcha is set for ContextEventCaptchaDetected and
	// ContextEventCaptchaResolved.
	Captcha *CaptchaEvent
	// Reaction is set for ContextEventReactionFired.
	Reaction *ReactionEvent
}

// ReactionEvent describes a reaction's click in a [ContextEventReactionFired]
// event.
type ReactionEvent struct {
	// ReactionId is the id [Page.AddReaction] or [CloudBrowser.AddReaction]
	// returned.
	ReactionId string
	// FrameId is the frame the match was found in.
	FrameId string
	// Success reports whether the click was carried out.
	Success bool
	// InterruptedCommand is the action running on the page when the
	// reaction acted ("fill", "type", "drag", ...), which waited until the
	// click was done. Empty if none was.
	InterruptedCommand string
	// Error is set iff Success is false, with the click's code:
	// "not_found" (the click target was gone), "occluded_no_reachable_point",
	// "occluded_after_evade" (something covered it) or "failed". A failed
	// reaction is used up like one that clicked.
	Error *CommandError
}

// CaptchaEvent describes a captcha solve in a [ContextEventCaptchaDetected] or
// [ContextEventCaptchaResolved] event. SolveId and InterruptedCommand are set
// for both; the other fields for the one they name.
type CaptchaEvent struct {
	// SolveId identifies the solve in both events and in the Captchas of the
	// actions it held.
	SolveId string
	// InterruptedCommand is the action running on the page when the captcha
	// appeared ("click", "fill", "type", "drag"), held until the solve ended.
	// Empty if none was.
	InterruptedCommand string

	// FrameId (detected) is the frame the challenge was found in.
	FrameId string
	// Trigger (detected) is "auto" (auto-solve) or "command"
	// ([Page.SolveCaptcha]).
	Trigger string
	// State (detected) is "solving" (the solve starts now),
	// "waiting_for_focus" (the page does not have the focus and auto-solve
	// waits for it, see [CaptchaAutoSolveOpts].Unfocused) or "skipped" (the
	// page reached [CaptchaAutoSolveOpts].MaxSolvesPerMinute; the resolved
	// event follows at once with code "limit_reached").
	State string

	// Success (resolved) reports whether the challenge was solved.
	Success bool
	// DurationMs (resolved) is the time from detection to the end, in
	// milliseconds, including any wait for the focus.
	DurationMs float64
	// Attempts (resolved) is how many interaction rounds the solve needed; 0
	// if it never got to interact.
	Attempts int32
	// InterruptedOutcome (resolved) is what InterruptedCommand does now; see
	// [CaptchaInterruption].Outcome.
	InterruptedOutcome string
	// Error (resolved) is set iff Success is false. Codes: "solve_failed",
	// "timeout", "limit_reached", "page_closed", "disabled" (auto-solve was
	// disabled before the solve started) and "cancelled" (a navigation of the
	// page started during the solve).
	Error *CommandError
}

// ContextEventHandler is called once per context event.
//
// Calls are sequential and in the order the browser reported the changes, so
// the handler needs no locking of its own. It runs on a goroutine the SDK
// owns, not the caller's. A handler that blocks for long makes the server
// drop this reader's backlog and start over with a snapshot.
type ContextEventHandler func(ContextEvent)

// ContextEvents is a running listener, returned by
// [CloudBrowser.ContextEvents]. Events go to the handler passed there; this
// handle only exists to stop listening and to report how it ended.
type ContextEvents struct {
	stream  generated.Browser_StreamContextEventsClient
	cancel  context.CancelFunc
	browser *CloudBrowser
	handler ContextEventHandler

	// finished is closed once the reader goroutine has returned, meaning no
	// further handler call can be in flight.
	finished chan struct{}

	mu  sync.Mutex
	err error
}

// ContextEvents registers a listener for the browser's pages: first a
// snapshot of every page, then each page opening, closing, taking the focus
// or changing its address or title. Use it to keep a tab list current
// without polling [CloudBrowser.Pages]. It also reports every captcha solve
// ([ContextEventCaptchaDetected], [ContextEventCaptchaResolved]) and every
// reaction's click ([ContextEventReactionFired]).
//
// This call returns as soon as the listener is registered; onEvent then fires
// in the background. Call [ContextEvents.Stop] when done.
//
// @param onEvent - called per event; see [ContextEventHandler] for the
//
//	ordering and blocking rules
//
// @returns *ContextEvents handle for stopping the listener and inspecting how
//
//	it ended
//
// A nil onEvent is rejected before anything is sent. Beyond that, reports only
// transport failures: listening has no semantic failure of its own.
//
// @example
//
//	tabs := map[string]*browserscale.PageInfo{}
//	listener, err := browser.ContextEvents(ctx, func(ev browserscale.ContextEvent) {
//	    switch ev.Kind {
//	    case browserscale.ContextEventSnapshot:
//	        clear(tabs)
//	        for _, p := range ev.Pages {
//	            tabs[p.Page.Id()] = p
//	        }
//	    case browserscale.ContextEventPageOpened:
//	        tabs[ev.Page.Id()] = ev.Opened
//	    case browserscale.ContextEventPageClosed:
//	        delete(tabs, ev.Page.Id())
//	    case browserscale.ContextEventPageUpdated:
//	        if t := tabs[ev.Page.Id()]; t != nil {
//	            t.Url, t.Title = ev.Url, ev.Title
//	        }
//	    }
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer listener.Stop()
func (c *CloudBrowser) ContextEvents(ctx context.Context, onEvent ContextEventHandler) (*ContextEvents, error) {
	if onEvent == nil {
		return nil, errors.New("browserscale.ContextEvents: onEvent must not be nil")
	}
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := c.client.StreamContextEvents(streamCtx, &generated.StreamContextEventsRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	ce := &ContextEvents{
		stream:   stream,
		cancel:   cancel,
		browser:  c,
		handler:  onEvent,
		finished: make(chan struct{}),
	}
	go ce.pump()
	return ce, nil
}

// Wait blocks until the listener ends — [ContextEvents.Stop], a cancelled
// context, a dead session or a transport failure — and returns
// [ContextEvents.Err].
func (ce *ContextEvents) Wait() error {
	<-ce.finished
	return ce.Err()
}

// Err reports why the listener ended. It returns nil while it is still
// running, and after a clean stop, a cancelled context, or the session ending
// normally.
func (ce *ContextEvents) Err() error {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.err
}

// Stop ends the listener. Idempotent, and safe to defer.
//
// Once it returns, the handler is no longer running and everything it wrote is
// visible to the calling goroutine. Do not call it from inside the handler:
// it waits for the handler to return. Cancel the context passed to
// [CloudBrowser.ContextEvents] there instead.
func (ce *ContextEvents) Stop() {
	ce.cancel()
	<-ce.finished
}

func (ce *ContextEvents) pump() {
	defer close(ce.finished)
	for {
		event, err := ce.stream.Recv()
		if err != nil {
			if !isStreamEnd(err) {
				ce.mu.Lock()
				ce.err = err
				ce.mu.Unlock()
			}
			return
		}
		if ev, ok := ce.browser.contextEventFromProto(event); ok {
			ce.handler(ev)
		}
	}
}

func (c *CloudBrowser) contextEventFromProto(e *generated.ContextEvent) (ContextEvent, bool) {
	switch ev := e.Event.(type) {
	case *generated.ContextEvent_Snapshot:
		pages := make([]*PageInfo, 0, len(ev.Snapshot.GetPages()))
		for _, p := range ev.Snapshot.GetPages() {
			pages = append(pages, c.pageInfoFromProto(p))
		}
		return ContextEvent{Kind: ContextEventSnapshot, Pages: pages}, true
	case *generated.ContextEvent_PageOpened:
		info := c.pageInfoFromProto(ev.PageOpened)
		if info == nil {
			return ContextEvent{}, false
		}
		return ContextEvent{Kind: ContextEventPageOpened, Opened: info, Page: info.Page}, true
	case *generated.ContextEvent_PageClosed:
		return ContextEvent{
			Kind:        ContextEventPageClosed,
			Page:        c.GetPage(ev.PageClosed.GetPageId()),
			FocusedPage: c.GetPage(ev.PageClosed.GetFocusedPageId()),
		}, true
	case *generated.ContextEvent_PageFocused:
		return ContextEvent{Kind: ContextEventPageFocused, Page: c.GetPage(ev.PageFocused.GetPageId())}, true
	case *generated.ContextEvent_PageUpdated:
		return ContextEvent{
			Kind:  ContextEventPageUpdated,
			Page:  c.GetPage(ev.PageUpdated.GetPageId()),
			Url:   ev.PageUpdated.GetUrl(),
			Title: ev.PageUpdated.GetTitle(),
		}, true
	case *generated.ContextEvent_CaptchaDetected:
		d := ev.CaptchaDetected
		return ContextEvent{
			Kind: ContextEventCaptchaDetected,
			Page: c.GetPage(d.GetPageId()),
			Captcha: &CaptchaEvent{
				SolveId:            d.GetSolveId(),
				InterruptedCommand: d.GetInterruptedCommand(),
				FrameId:            d.GetFrameId(),
				Trigger:            d.GetTrigger(),
				State:              d.GetState(),
			},
		}, true
	case *generated.ContextEvent_CaptchaResolved:
		r := ev.CaptchaResolved
		var cerr *CommandError
		if e := r.GetError(); e != nil {
			cerr = &CommandError{Command: "captcha", Code: e.GetCode(), Message: e.GetMessage()}
		}
		return ContextEvent{
			Kind: ContextEventCaptchaResolved,
			Page: c.GetPage(r.GetPageId()),
			Captcha: &CaptchaEvent{
				SolveId:            r.GetSolveId(),
				InterruptedCommand: r.GetInterruptedCommand(),
				Success:            r.GetSuccess(),
				DurationMs:         r.GetDurationMs(),
				Attempts:           r.GetAttempts(),
				InterruptedOutcome: r.GetInterruptedOutcome(),
				Error:              cerr,
			},
		}, true
	case *generated.ContextEvent_ReactionFired:
		r := ev.ReactionFired
		var cerr *CommandError
		if e := r.GetError(); e != nil {
			cerr = &CommandError{Command: "reaction", Code: e.GetCode(), Message: e.GetMessage()}
		}
		return ContextEvent{
			Kind: ContextEventReactionFired,
			Page: c.GetPage(r.GetPageId()),
			Reaction: &ReactionEvent{
				ReactionId:         r.GetReactionId(),
				FrameId:            r.GetFrameId(),
				Success:            r.GetSuccess(),
				InterruptedCommand: r.GetInterruptedCommand(),
				Error:              cerr,
			},
		}, true
	}
	return ContextEvent{}, false
}
