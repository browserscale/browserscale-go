package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// DevTools / live-UI helpers
//
// These RPCs back the interactive panel in the user frontend (DOM tree,
// hover-highlighting, click-to-inspect, paste/copy, software keyboard on the
// WebRTC stream). They are perfectly valid for automation scripts too, but
// the primary consumer is the live-browser UI.

// ── DOM helpers ──

// GetDOMHash returns sha256[:8] of the full-tree DOM JSON for cheap
// polling-based change detection.
//
// Computing a hash is much cheaper than transferring the full tree —
// pair this with [CloudBrowser.GetDOM] only when the hash differs from
// your last snapshot.
//
// @param frameId - id of the frame to hash; empty targets the main frame
//
// @returns 16-char hex string (the first 8 bytes of sha256 of the DOM JSON)
//
// Reports only transport failures. The hash is computed from a serialized tree,
// so there is no semantic failure of its own and no error codes to branch on.
//
// @example
//
//	hash, err := browser.GetDOMHash(ctx, "")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if hash != lastHash {
//	    // DOM changed → re-fetch
//	}
func (c *CloudBrowser) GetDOMHash(ctx context.Context, frameId string) (string, error) {
	resp, err := c.client.GetDOMHash(ctx, &generated.GetDOMHashRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		FrameId: strPtr(frameId),
	})
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}

// InspectAtPosition hit-tests at the viewport-relative (x, y) and returns
// the topmost element under that point.
//
// Mirrors what the live-UI overlay does on hover. Elements with
// pointer-events:none are skipped — the result is the actual click target,
// not the visually-topmost node.
//
// @param x - viewport-relative x in CSS pixels
// @param y - viewport-relative y in CSS pixels
//
// @returns *InspectResult with the resolved backendNodeId, frameId, tag
//
//	name, trimmed textContent, visibility and bounds
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	res, err := browser.InspectAtPosition(ctx, 200, 300)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(res.TagName, res.TextContent)
func (c *CloudBrowser) InspectAtPosition(ctx context.Context, x, y float64) (*InspectResult, error) {
	resp, err := c.client.InspectAtPosition(ctx, &generated.InspectAtPositionRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		X: x,
		Y: y,
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("inspectAtPosition", resp.GetError()); e != nil {
		return nil, e
	}
	return &InspectResult{
		BackendNodeId: resp.BackendNodeId,
		FrameId:       resp.FrameId,
		TagName:       resp.TagName,
		TextContent:   resp.TextContent,
		IsVisible:     resp.IsVisible,
		Bounds:        rectFromProto(resp.Bounds),
	}, nil
}

// HighlightNode paints a debug overlay over the node identified by
// backendNodeId.
//
// Useful for visual debugging of agent flows — the overlay stays until the
// next call. Pass backendNodeId <= 0 to clear any current highlights.
//
// @param backendNodeId - id of the node to highlight, or <= 0 to clear
// @param frameId - id of the frame the node lives in; empty targets the main frame
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	if err := browser.HighlightNode(ctx, res.BackendNodeId, res.FrameId); err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) HighlightNode(ctx context.Context, backendNodeId int32, frameId string) error {
	resp, err := c.client.HighlightNode(ctx, &generated.HighlightNodeRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		BackendNodeId: backendNodeId,
		FrameId:       strPtr(frameId),
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("highlight", resp.GetError())
}

// ── Keyboard / IME / selection ──

// InsertText pastes text at the current caret using IME-style input.
//
// No individual key events are dispatched; the entire string is committed
// at once via Input.insertText. Whatever element currently has focus
// receives the text. Use [CloudBrowser.Click] or [CloudBrowser.Fill] first
// if you need a specific element to be focused.
//
// @param text - the text to insert at the caret
//
// @throws no_focus - nothing in the page holds focus, so there is no caret to
// insert at; click the field first
// @throws busy - another action is already running on this page
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	if err := browser.InsertText(ctx, "hello world"); err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) InsertText(ctx context.Context, text string) error {
	resp, err := c.client.InsertText(ctx, &generated.InsertTextRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Text: text,
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("insertText", resp.GetError())
}

// Type types text into the currently focused element as a per-key stream of
// real keyboard events (keyDown/char/keyUp with the context's QWERTZ/QWERTY
// layout and human cadence) — unlike [CloudBrowser.InsertText], a single
// IME-style commit with no key events.
//
// Type is intentionally UNtargeted and loose: it does not locate or focus any
// element and does NOT pin focus, so the page is free to route keys and move
// focus between fields mid-stream — ideal for one-time-code / OTP inputs that
// auto-advance to the next box on each digit. To type one specific field that
// must stay focused for the whole value, use [CloudBrowser.Fill] instead
// (strict, target-bound, per-key focus-verified).
//
// Nothing is focused for you: [CloudBrowser.Click] (or Fill) the field first,
// or otherwise ensure focus, before calling Type.
//
// @param text - the text to type as real key events
// @param clearFirst - when true, clears the focused field (Ctrl+A, Delete) first
//
// Type has no semantic failure of its own: the keys land wherever focus happens
// to be, so there is no target it can miss. Only the page or context being torn
// down mid-stream surfaces, and that is a transport error rather than a code.
//
// @example
//
//	// OTP field that auto-advances across boxes.
//	_, _ = browser.Click(ctx, browserscale.CSS("input.otp-0"))
//	if err := browser.Type(ctx, "123456", false); err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) Type(ctx context.Context, text string, clearFirst bool) error {
	req := &generated.TypeRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Text: text,
	}
	if clearFirst {
		t := true
		req.ClearFirst = &t
	}
	resp, err := c.client.Type(ctx, req)
	if err != nil {
		return err
	}
	return commandErrorFrom("type", resp.GetError())
}

// PressKey fires a single key-down event.
//
// Only the keydown half is dispatched — pair with [CloudBrowser.ReleaseKey]
// for a full press cycle. The event targets whichever element currently has
// focus.
//
// @param key - DOM KeyboardEvent.key value (e.g. "Enter", "a", "ArrowLeft")
// @param code - DOM KeyboardEvent.code value (e.g. "Enter", "KeyA"); empty falls back to key
// @param modifiers - bit-flag combination: Alt=1, Ctrl=2, Meta=4, Shift=8
// @param location - DOM KeyboardEvent.location: 0=standard, 1=left, 2=right, 3=numpad
//
// @throws no_focus - nothing in the page holds focus, so the key has nowhere to
// go; click the field first
// @throws busy - another action is already running on this page
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	// Ctrl+A
//	_ = browser.PressKey(ctx, "a", "KeyA", 2, 0)
//	_ = browser.ReleaseKey(ctx, "a", "KeyA", 2, 0)
func (c *CloudBrowser) PressKey(ctx context.Context, key, code string, modifiers, location int32) error {
	resp, err := c.client.PressKey(ctx, &generated.PressKeyRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Key:       key,
		Code:      strPtr(code),
		Modifiers: intPtr(modifiers),
		Location:  intPtr(location),
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("pressKey", resp.GetError())
}

// ReleaseKey fires a single key-up event.
//
// Mirror of [CloudBrowser.PressKey]. Same parameter semantics; use this to
// close a press cycle that was started with PressKey.
//
// @inheritDoc [CloudBrowser.PressKey]
//
// @example
//
//	_ = browser.PressKey(ctx, "Shift", "ShiftLeft", 0, 1)
//	_ = browser.ReleaseKey(ctx, "Shift", "ShiftLeft", 0, 1)
func (c *CloudBrowser) ReleaseKey(ctx context.Context, key, code string, modifiers, location int32) error {
	resp, err := c.client.ReleaseKey(ctx, &generated.ReleaseKeyRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Key:       key,
		Code:      strPtr(code),
		Modifiers: intPtr(modifiers),
		Location:  intPtr(location),
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("releaseKey", resp.GetError())
}

// GetSelection returns the current text selection.
//
// Walks every frame and returns the first non-empty selection found —
// useful for "copy what the user highlighted" flows. Returns an empty
// string when nothing is selected anywhere.
//
// @returns the selected text, or "" when nothing is selected
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	sel, err := browser.GetSelection(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("user selected:", sel)
func (c *CloudBrowser) GetSelection(ctx context.Context) (string, error) {
	resp, err := c.client.GetSelection(ctx, &generated.GetSelectionRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return "", err
	}
	if e := commandErrorFrom("getSelection", resp.GetError()); e != nil {
		return "", e
	}
	return resp.Text, nil
}
