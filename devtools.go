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
// @throws UNKNOWN_ERROR - the hash could not be computed
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
// @throws UNKNOWN_ERROR - the hit-test failed
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
// @throws UNKNOWN_ERROR - the highlight could not be applied
//
// @example
//
//	if err := browser.HighlightNode(ctx, res.BackendNodeId, res.FrameId); err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) HighlightNode(ctx context.Context, backendNodeId int32, frameId string) error {
	_, err := c.client.HighlightNode(ctx, &generated.HighlightNodeRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		BackendNodeId: backendNodeId,
		FrameId:       strPtr(frameId),
	})
	return err
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
// @throws UNKNOWN_ERROR - the text could not be inserted
//
// @example
//
//	if err := browser.InsertText(ctx, "hello world"); err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) InsertText(ctx context.Context, text string) error {
	_, err := c.client.InsertText(ctx, &generated.InsertTextRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Text: text,
	})
	return err
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
// @throws UNKNOWN_ERROR - the event could not be dispatched
//
// @example
//
//	// Ctrl+A
//	_ = browser.PressKey(ctx, "a", "KeyA", 2, 0)
//	_ = browser.ReleaseKey(ctx, "a", "KeyA", 2, 0)
func (c *CloudBrowser) PressKey(ctx context.Context, key, code string, modifiers, location int32) error {
	_, err := c.client.PressKey(ctx, &generated.PressKeyRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Key:       key,
		Code:      strPtr(code),
		Modifiers: intPtr(modifiers),
		Location:  intPtr(location),
	})
	return err
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
	_, err := c.client.ReleaseKey(ctx, &generated.ReleaseKeyRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Key:       key,
		Code:      strPtr(code),
		Modifiers: intPtr(modifiers),
		Location:  intPtr(location),
	})
	return err
}

// GetSelection returns the current text selection.
//
// Walks every frame and returns the first non-empty selection found —
// useful for "copy what the user highlighted" flows. Returns an empty
// string when nothing is selected anywhere.
//
// @returns the selected text, or "" when nothing is selected
//
// @throws UNKNOWN_ERROR - the selection could not be read
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
	return resp.Text, nil
}
