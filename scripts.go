package browserscale

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/browserscale/browserscale-go/generated"
)

// ScriptLogEntry is one console.* call from a script.
type ScriptLogEntry struct {
	// Level is "info", "warning" or "error", from console.log / .warn / .error.
	Level string
	// Message holds the logged arguments, already stringified the way console
	// does it.
	Message string
	// Timestamp is when the script printed the line, stamped in the browser.
	Timestamp time.Time
}

// ScriptResult is the outcome of a blocking [CloudBrowser.RunScript].
type ScriptResult struct {
	// Success is false when the script failed to compile or threw; Result then
	// holds the message.
	Success bool
	// Result is the return value as JSON, or "undefined" when the script
	// returned nothing. On failure it is the error message.
	Result string
	// RunId names the run. It arrives with the reply, so it is only useful after
	// the fact — to match up log lines a separate follower already saw.
	RunId string
	// Log is everything the script printed, in order.
	Log []ScriptLogEntry
	// Truncated is true when the script printed more than the reply holds, in
	// which case Log is the tail of the output rather than all of it.
	Truncated bool
}

// ScriptFinished says how a run ended.
type ScriptFinished struct {
	// Success is false when the script failed to compile or threw; Result then
	// holds the message.
	Success bool
	// Result is the return value as JSON, or the error message.
	Result string
	// Stopped is true when the run was cancelled, or the session went away under
	// it, rather than the script returning on its own.
	Stopped bool
}

// ScriptRunInfo is one run still in flight, as
// [CloudBrowser.ListScriptRuns] reports it.
type ScriptRunInfo struct {
	RunId string
	// Running is how long the run has been going.
	Running time.Duration
}

// ScriptEvent is one item on a session's script event stream. Exactly one of
// Log and Finished is set.
type ScriptEvent struct {
	// RunId is the run that produced this event.
	RunId string
	// Log is a console line the script printed.
	Log *ScriptLogEntry
	// Finished marks the end of the run. No further event for that run follows.
	Finished *ScriptFinished
}

// ScriptEventHandler is called once per script event.
//
// Calls are sequential and in the order the browser produced them, so a run's
// last log line always arrives before its Finished and the handler needs no
// locking of its own. It runs on a goroutine the SDK owns, not the caller's.
//
// Blocking here stalls delivery: the server buffers a bounded number of events
// per reader and then drops its oldest, which [ScriptRun.Dropped] reports. Hand
// slow work to another goroutine.
type ScriptEventHandler func(ScriptEvent)

// RunScript runs source in the session's browser and waits for it to finish.
//
// The script runs beside the browser, in a V8 isolate of its own rather than in
// the page, and reaches the document through the engine: a cross-origin iframe
// is read as plain `contentDocument` with no frame ids anywhere, values come back
// as live objects it can assign to rather than snapshots, an element can be
// handed straight to `browser.click`, and the page sees nothing injected. Steps
// cost microseconds rather than network round trips, so work that is chatty by
// nature — polling for a selector, walking a list, following pagination — is
// affordable there. A guide for it is still to come.
//
// This blocks for as long as the script runs, and cannot be bounded: the run id
// needed to cancel only arrives with the reply. Cancelling ctx abandons the wait
// but not the run. Use [CloudBrowser.StartScript] when the script may outlive
// the caller's patience, or [CloudBrowser.StopScripts] to abandon what this
// session is running.
//
// @param source - JavaScript to execute; its return value comes back as JSON
//
// @returns *ScriptResult with the return value and the script's whole console
//
//	output. A script that threw is reported as Success false, not as an error
//
// Reports only transport failures. A script that fails to compile or throws is
// not an error here: the returned ScriptResult has Success=false and Result holds
// the message, so a broken script stays distinguishable from a broken connection.
//
// @example
//
//	result, err := browser.RunScript(ctx, `
//	    await browser.navigate("https://example.com");
//	    const items = [];
//	    for (const el of await browser.getDOM().querySelectorAll("h1")) {
//	        items.push(el.textContent);
//	    }
//	    return items;
//	`)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Success, result.Result)
func (c *CloudBrowser) RunScript(ctx context.Context, source string) (*ScriptResult, error) {
	resp, err := c.client.RunScript(ctx, &generated.RunScriptRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Source: source,
	})
	if err != nil {
		return nil, err
	}
	return &ScriptResult{
		Success:   resp.Success,
		Result:    resp.Result,
		RunId:     resp.RunId,
		Log:       scriptLogFromProto(resp.Log),
		Truncated: resp.Truncated,
	}, nil
}

// ScriptRun is a script running in the background, returned by
// [CloudBrowser.StartScript]. Its output is delivered to the handler passed
// there; this handle exists to wait for the outcome and to cancel the run.
type ScriptRun struct {
	browser *CloudBrowser
	stream  generated.Browser_StreamScriptEventsClient
	cancel  context.CancelFunc
	handler ScriptEventHandler
	runId   string

	// finished is closed once the reader goroutine has returned, meaning no
	// further handler call can be in flight.
	finished chan struct{}
	// ended is closed when the run's own Finished event arrives, which may be
	// long before the stream closes.
	ended chan struct{}

	mu      sync.Mutex
	err     error
	dropped uint64
	outcome *ScriptFinished
}

// StartScript launches source in the session's browser and returns as soon as
// the run is under way.
//
// The counterpart to [CloudBrowser.RunScript], for scripts that are not worth
// waiting on: a watcher that runs for the life of the session, work that should
// survive this process. Output arrives at onEvent while the caller gets on with
// something else, and [ScriptRun.Wait] collects the outcome if it is wanted.
//
// Subscribing has to happen before the launch, because a detached run's output
// is not kept anywhere — the browser rejects a start with nobody listening
// rather than discard the script's log and result. This call does both in that
// order, so nothing the script prints is missed.
//
// @param source - JavaScript to execute
// @param onEvent - called per log line and once for the outcome; see
//
//	[ScriptEventHandler] for the ordering and blocking rules
//
// @returns *ScriptRun handle for awaiting or cancelling the run
//
// A nil onEvent is rejected before anything is sent. Beyond that, reports only
// transport failures: a script that fails to compile or throws surfaces on the
// run itself rather than here.
//
// @example
//
//	run, err := browser.StartScript(ctx, source, func(ev browserscale.ScriptEvent) {
//	    if ev.Log != nil {
//	        fmt.Println(ev.Log.Level, ev.Log.Message)
//	    }
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	outcome, err := run.Wait(ctx)
func (c *CloudBrowser) StartScript(ctx context.Context, source string, onEvent ScriptEventHandler) (*ScriptRun, error) {
	if onEvent == nil {
		return nil, errors.New("browserscale.StartScript: onEvent must not be nil")
	}

	// Subscribe unfiltered: the run id this handle filters on does not exist
	// yet. Events for it pile up in the server's per-reader buffer between the
	// subscription and the pump, which is what that buffer is for.
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := c.client.StreamScriptEvents(streamCtx, &generated.StreamScriptEventsRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	// Opening a stream does not wait for the server to start handling it. For a
	// capture that would only cost the first few events; here it would fail the
	// launch outright, since the browser refuses to start a run before a
	// subscription exists. The server sends its headers once subscribed.
	if _, err := stream.Header(); err != nil {
		cancel()
		return nil, err
	}

	runId, err := c.client.StartScript(streamCtx, &generated.StartScriptRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Source: source,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	run := &ScriptRun{
		browser:  c,
		stream:   stream,
		cancel:   cancel,
		handler:  onEvent,
		runId:    runId.RunId,
		finished: make(chan struct{}),
		ended:    make(chan struct{}),
	}
	go run.pump()
	return run, nil
}

// RunId is the id the browser gave this run. Pass it to
// [CloudBrowser.StopScripts] to cancel the run from elsewhere, or to
// [CloudBrowser.FollowScript] to watch it from another process.
func (r *ScriptRun) RunId() string { return r.runId }

// Wait blocks until the run ends and returns how it ended.
//
// A script that threw is an outcome, not an error: it comes back with Success
// false. An error means the run's fate is unknown — the stream broke, the
// session died, or ctx expired before the script finished.
//
// @returns *ScriptFinished describing how the script ended
//
// Reports only transport failures - the connection dying, or the context being
// cancelled while waiting. A script that threw is a normal outcome and arrives in
// the returned ScriptFinished.
func (r *ScriptRun) Wait(ctx context.Context) (*ScriptFinished, error) {
	select {
	case <-r.ended:
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.outcome, nil
	case <-r.finished:
		// The stream ended without an outcome for this run.
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.outcome != nil {
			return r.outcome, nil
		}
		if r.err != nil {
			return nil, r.err
		}
		return nil, errors.New("browserscale.ScriptRun.Wait: stream ended before the run did")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Stop cancels the run and detaches this reader. Idempotent, and safe to defer.
//
// A script that is executing is interrupted; one parked on an await unwinds at
// its next operation in the page. Either way the handler sees a Finished with
// Stopped set, unless the local reader is torn down first.
//
// ctx covers the cancel call, so pass a live one: the context the run was
// started with may already be cancelled by the time you stop.
//
// Reports only transport failures, and the local reader is detached regardless.
// Cancelling a run that has already finished is a no-op rather than a failure.
func (r *ScriptRun) Stop(ctx context.Context) error {
	_, err := r.browser.StopScripts(ctx, r.runId)
	r.cancel()
	select {
	case <-r.finished:
	case <-ctx.Done():
	}
	return err
}

// Detach stops reading this run's output without cancelling the run. The script
// keeps going with nobody watching, which is what makes a detached run outlive
// the process that started it.
func (r *ScriptRun) Detach() {
	r.cancel()
	<-r.finished
}

// Err reports why the stream ended, or nil while it is still open and after a
// clean stop, a cancelled context, or the session ending normally.
func (r *ScriptRun) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Dropped reports how many events the server discarded because this reader fell
// behind. Anything above zero means the log has holes: make the handler cheaper,
// or have the script print less.
func (r *ScriptRun) Dropped() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

// pump reads the gRPC stream and calls the handler for each event belonging to
// this run. Filtering happens here rather than server-side because the
// subscription had to exist before the run id did.
func (r *ScriptRun) pump() {
	defer close(r.finished)
	for {
		event, err := r.stream.Recv()
		if err != nil {
			if !isStreamEnd(err) {
				r.mu.Lock()
				r.err = err
				r.mu.Unlock()
			}
			return
		}
		if event.Dropped > 0 {
			r.mu.Lock()
			r.dropped = event.Dropped
			r.mu.Unlock()
		}
		ev, ok := scriptEventFromProto(event)
		if !ok || ev.RunId != r.runId {
			continue
		}
		r.handler(ev)
		if ev.Finished != nil {
			r.mu.Lock()
			r.outcome = ev.Finished
			r.mu.Unlock()
			close(r.ended)
			return
		}
	}
}

// ScriptFollow is a read-only view of script output, returned by
// [CloudBrowser.FollowScript].
type ScriptFollow struct {
	stream  generated.Browser_StreamScriptEventsClient
	cancel  context.CancelFunc
	handler ScriptEventHandler

	finished chan struct{}

	mu      sync.Mutex
	err     error
	dropped uint64
}

// FollowScript watches script output in a session without starting anything.
//
// For the case [CloudBrowser.StartScript] cannot cover: a run somebody else
// launched, or one this process started before it restarted. Several readers can
// watch the same session, each with its own buffer.
//
// Only output produced from now on arrives — lines printed before the
// subscription existed are not kept. A run that has already finished is
// therefore invisible here; [CloudBrowser.ListScriptRuns] is how you tell that
// apart from a run that is merely quiet.
//
// @param runId - run to follow, or "" to follow every run in the session
// @param onEvent - called per event; see [ScriptEventHandler] for the ordering
//
//	and blocking rules
//
// @returns *ScriptFollow handle for stopping the subscription
//
// A nil onEvent is rejected before anything is sent. Beyond that, reports only
// transport failures: opening the subscription has no semantic failure of its own.
//
// @example
//
//	follow, err := browser.FollowScript(ctx, runId, func(ev browserscale.ScriptEvent) {
//	    if ev.Log != nil { fmt.Println(ev.Log.Message) }
//	})
//	if err != nil { log.Fatal(err) }
//	defer follow.Stop()
func (c *CloudBrowser) FollowScript(ctx context.Context, runId string, onEvent ScriptEventHandler) (*ScriptFollow, error) {
	if onEvent == nil {
		return nil, errors.New("browserscale.FollowScript: onEvent must not be nil")
	}
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := c.client.StreamScriptEvents(streamCtx, &generated.StreamScriptEventsRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		RunId: runId,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	if _, err := stream.Header(); err != nil {
		cancel()
		return nil, err
	}
	f := &ScriptFollow{
		stream:   stream,
		cancel:   cancel,
		handler:  onEvent,
		finished: make(chan struct{}),
	}
	go f.pump()
	return f, nil
}

// Wait blocks until the subscription ends — [ScriptFollow.Stop], a cancelled
// context, a dead session or a transport failure — and returns
// [ScriptFollow.Err].
func (f *ScriptFollow) Wait() error {
	<-f.finished
	return f.Err()
}

// Stop ends the subscription. Idempotent, and safe to defer. It never cancels a
// run: other readers, and the script itself, are unaffected.
//
// Once it returns, the handler is no longer running and everything it wrote is
// visible to the calling goroutine.
func (f *ScriptFollow) Stop() {
	f.cancel()
	<-f.finished
}

// Err reports why the subscription ended, or nil while it is still open and
// after a clean stop.
func (f *ScriptFollow) Err() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// Dropped reports how many events the server discarded because this reader fell
// behind.
func (f *ScriptFollow) Dropped() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dropped
}

func (f *ScriptFollow) pump() {
	defer close(f.finished)
	for {
		event, err := f.stream.Recv()
		if err != nil {
			if !isStreamEnd(err) {
				f.mu.Lock()
				f.err = err
				f.mu.Unlock()
			}
			return
		}
		if event.Dropped > 0 {
			f.mu.Lock()
			f.dropped = event.Dropped
			f.mu.Unlock()
		}
		if ev, ok := scriptEventFromProto(event); ok {
			f.handler(ev)
		}
	}
}

// StopScripts cancels runs in the session and reports how many it ended.
//
// An empty runId cancels every run in the session, which is the only form
// available to a caller that never learned an id — notably one abandoning a
// blocking [CloudBrowser.RunScript].
//
// @param runId - run to cancel, or "" for all of them
//
// @returns int how many runs were cancelled; 0 when the id named nothing in
//
//	flight
//
// Reports only transport failures. Cancelling runs that have already finished, or
// none at all, is a no-op rather than a failure - read the returned count to learn
// how many were actually stopped.
//
// @example
//
//	_, err := browser.StopScripts(ctx, "") // abandon everything running
func (c *CloudBrowser) StopScripts(ctx context.Context, runId string) (int, error) {
	resp, err := c.client.StopScripts(ctx, &generated.StopScriptsRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		RunId: runId,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Stopped), nil
}

// ListScriptRuns reports the scripts still running in the session.
//
// Only runs in flight — a finished run is reported once on the event stream and
// then forgotten, so this is not a history. Its use is finding work this caller
// did not start: a script a previous process left behind, which
// [CloudBrowser.StopScripts] needs an id to name.
//
// @returns []ScriptRunInfo one entry per run still executing
//
// Reports only transport failures - a dead session, a broken connection. This
// call has no semantic failure of its own, so there are no error codes to branch
// on.
//
// @example
//
//	runs, err := browser.ListScriptRuns(ctx)
//	if err != nil { log.Fatal(err) }
//	for _, run := range runs {
//	    fmt.Println(run.RunId, run.Running)
//	}
func (c *CloudBrowser) ListScriptRuns(ctx context.Context) ([]ScriptRunInfo, error) {
	resp, err := c.client.ListScriptRuns(ctx, &generated.ListScriptRunsRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	runs := make([]ScriptRunInfo, 0, len(resp.Runs))
	for _, run := range resp.Runs {
		runs = append(runs, ScriptRunInfo{
			RunId:   run.RunId,
			Running: time.Duration(run.RunningMs) * time.Millisecond,
		})
	}
	return runs, nil
}

func scriptLogEntryFromProto(e *generated.ScriptLogEntry) ScriptLogEntry {
	return ScriptLogEntry{
		Level:     e.Level,
		Message:   e.Message,
		Timestamp: time.UnixMilli(e.Timestamp),
	}
}

func scriptLogFromProto(entries []*generated.ScriptLogEntry) []ScriptLogEntry {
	out := make([]ScriptLogEntry, 0, len(entries))
	for _, e := range entries {
		if e == nil {
			continue
		}
		out = append(out, scriptLogEntryFromProto(e))
	}
	return out
}

// scriptEventFromProto converts one stream item. The bool is false for an event
// carrying neither variant, which a newer server could send and an older client
// should ignore rather than report as an empty log line.
func scriptEventFromProto(event *generated.ScriptEvent) (ScriptEvent, bool) {
	switch payload := event.Event.(type) {
	case *generated.ScriptEvent_Log:
		if payload.Log == nil || payload.Log.Line == nil {
			return ScriptEvent{}, false
		}
		entry := scriptLogEntryFromProto(payload.Log.Line)
		return ScriptEvent{RunId: payload.Log.RunId, Log: &entry}, true
	case *generated.ScriptEvent_Finished:
		if payload.Finished == nil {
			return ScriptEvent{}, false
		}
		return ScriptEvent{
			RunId: payload.Finished.RunId,
			Finished: &ScriptFinished{
				Success: payload.Finished.Success,
				Result:  payload.Finished.Result,
				Stopped: payload.Finished.Stopped,
			},
		}, true
	}
	return ScriptEvent{}, false
}
