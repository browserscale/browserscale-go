package browserscale

import (
	"context"
	"errors"

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
// pair this with [Page.GetDOM] only when the hash differs from
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
//	hash, err := page.GetDOMHash(ctx, "")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if hash != lastHash {
//	    // DOM changed → re-fetch
//	}
func (p *Page) GetDOMHash(ctx context.Context, frameId string) (string, error) {
	resp, err := p.client.GetDOMHash(ctx, &generated.GetDOMHashRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
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
// @returns *InspectResult with the element's Ref and Frame, its address in
//
//	the DOM mirror (BackendNodeId, FrameId), tag name, trimmed textContent,
//	visibility and bounds
//
// Reports only transport failures - a dead session, a page that is gone, a
// broken connection. This call has no semantic failure of its own, so there are
// no error codes to branch on.
//
// @example
//
//	res, err := page.InspectAtPosition(ctx, 200, 300)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(res.TagName, res.TextContent)
//	if res.Ref != nil {
//	    _, _ = page.Click(ctx, res.Ref) // act on what is under the point
//	}
func (p *Page) InspectAtPosition(ctx context.Context, x, y float64) (*InspectResult, error) {
	resp, err := p.client.InspectAtPosition(ctx, &generated.InspectAtPositionRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
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
		Ref:           refLocator(resp.Ref, resp.Frame),
		Frame:         frameRefFromProto(resp.Frame),
		BackendNodeId: resp.BackendNodeId,
		FrameId:       resp.FrameId,
		TagName:       resp.TagName,
		TextContent:   resp.TextContent,
		IsVisible:     resp.IsVisible,
		Bounds:        rectFromProto(resp.Bounds),
	}, nil
}

// Highlight paints a debug overlay over the element a reference names.
//
// Useful for visual debugging of agent flows: mark what a wait found or what
// a click is about to hit. At most one element is marked on the whole page;
// the overlay stays until the next call. Pass nil to clear it. The overlay is
// drawn by the browser, not by the page: it changes nothing in the document
// and scripts cannot see it.
//
// @param ref - the Ref of a result (or [Ref] for one read from an
// observation); nil clears the highlight
//
// @throws [StaleRefError] - the reference's element is gone
//
// @example
//
//	res, _ := page.Wait(ctx, browserscale.CSS("button.submit"))
//	if err := page.Highlight(ctx, res.Element.Ref); err != nil {
//	    log.Fatal(err)
//	}
func (p *Page) Highlight(ctx context.Context, ref *Locator) error {
	req := &generated.HighlightNodeRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
	}
	if ref != nil {
		if ref.ref == "" {
			return errors.New("browserscale.Highlight: takes the Ref of a result (or nil to clear); look the element up with Wait first")
		}
		req.Ref = strPtr(ref.ref)
	}
	resp, err := p.client.HighlightNode(ctx, req)
	if err != nil {
		return err
	}
	if e := commandErrorFrom("highlight", resp.GetError()); e != nil {
		return p.checked(ctx, ref, e)
	}
	return nil
}

// NodeRef turns the address of a DOM mirror node into a reference that
// [Page.Click], [Page.Fill] and every other call with a target takes.
//
// The mirror ([Page.MirrorDom]) and [Page.GetDOM] name a node by its
// backendNodeId and the frame it lives in. Actions take a reference instead,
// which also names the document, so that a node id can never be mistaken for
// a node of whatever the frame shows next. This is the bridge between the
// two: the reference names the node in the document its frame shows now.
// Whether the node still exists is not checked here; a reference made from an
// outdated id fails with a [*StaleRefError] when it is used.
//
// @param backendNodeId - the node's DomNode.BackendNodeId
// @param frameId - the node's DomNode.FrameId; empty is the main frame
//
// @returns *Locator naming the node, usable as a target or a container
//
// @throws not_found - the frame is not on the page any more
//
// @see [CommandError] for recovering the code with errors.As
// @see [DomMirror.Locator] for the same from a mirror node
//
// @example
//
//	button, err := page.NodeRef(ctx, node.BackendNodeId, node.FrameId)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_, _ = page.Click(ctx, button)
func (p *Page) NodeRef(ctx context.Context, backendNodeId int32, frameId string) (*Locator, error) {
	resp, err := p.client.GetNodeRef(ctx, &generated.GetNodeRefRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		BackendNodeId: backendNodeId,
		FrameId:       strPtr(frameId),
	})
	if err != nil {
		return nil, err
	}
	if e := commandErrorFrom("nodeRef", resp.GetError()); e != nil {
		return nil, e
	}
	return refLocator(resp.GetRef(), resp.GetFrame()), nil
}

// HighlightNode paints a debug overlay over a node of the DOM mirror,
// identified the way the mirror identifies it: by backendNodeId and frame.
// For an element a result named, use [Page.Highlight] with its Ref.
//
// The overlay stays until the next call. Pass backendNodeId <= 0 to clear any
// current highlights.
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
//	if err := page.HighlightNode(ctx, node.BackendNodeId, node.FrameId); err != nil {
//	    log.Fatal(err)
//	}
func (p *Page) HighlightNode(ctx context.Context, backendNodeId int32, frameId string) error {
	resp, err := p.client.HighlightNode(ctx, &generated.HighlightNodeRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
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
// receives the text. Use [Page.Click] or [Page.Fill] first
// if you need a specific element to be focused.
//
// @param text - the text to insert at the caret
//
// @throws no_focus - nothing in the page holds focus, so there is no caret to
// insert at; click the field first
// @throws busy - another action is already running on this page, or a captcha
// is being solved there (see [CloudBrowser.EnableCaptchaAutoSolve])
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	if err := page.InsertText(ctx, "hello world"); err != nil {
//	    log.Fatal(err)
//	}
func (p *Page) InsertText(ctx context.Context, text string) error {
	resp, err := p.client.InsertText(ctx, &generated.InsertTextRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		Text: text,
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("insertText", resp.GetError())
}

// Type types text into the currently focused element as a per-key stream of
// real keyboard events (keyDown/char/keyUp on the keyboard layout of the
// session's region, with human cadence) — unlike [Page.InsertText], a single
// IME-style commit with no key events.
//
// Type is intentionally UNtargeted and loose: it does not locate or focus any
// element and does NOT pin focus, so the page is free to route keys and move
// focus between fields mid-stream — ideal for one-time-code / OTP inputs that
// auto-advance to the next box on each digit. To type one specific field that
// must stay focused for the whole value, use [Page.Fill] instead
// (strict, target-bound, per-key focus-verified).
//
// Nothing is focused for you: [Page.Click] (or Fill) the field first,
// or otherwise ensure focus, before calling Type.
//
// @param text - the text to type as real key events
// @param clearFirst - when true, clears the focused field (Ctrl+A, Delete) first
//
// Type has no target it can miss: the keys land wherever focus happens to be.
// A captcha solve holds it between two keys (see
// [CloudBrowser.EnableCaptchaAutoSolve]); afterwards it puts the caret back and
// types the rest. The page or context being torn down mid-stream is a
// transport error rather than a code.
//
// @throws captcha_failed - a captcha held the typing and could not be solved
//
// @example
//
//	// OTP field that auto-advances across boxes.
//	_, _ = page.Click(ctx, browserscale.CSS("input.otp-0"))
//	if err := page.Type(ctx, "123456", false); err != nil {
//	    log.Fatal(err)
//	}
func (p *Page) Type(ctx context.Context, text string, clearFirst bool) error {
	req := &generated.TypeRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
		Text: text,
	}
	if clearFirst {
		t := true
		req.ClearFirst = &t
	}
	resp, err := p.client.Type(ctx, req)
	if err != nil {
		return err
	}
	return commandErrorFrom("type", resp.GetError())
}

// PressKey fires a single key-down event.
//
// Only the keydown half is dispatched — pair with [Page.ReleaseKey]
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
// @throws busy - another action is already running on this page, or a captcha
// is being solved there (see [CloudBrowser.EnableCaptchaAutoSolve])
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	// Ctrl+A
//	_ = page.PressKey(ctx, "a", "KeyA", 2, 0)
//	_ = page.ReleaseKey(ctx, "a", "KeyA", 2, 0)
func (p *Page) PressKey(ctx context.Context, key, code string, modifiers, location int32) error {
	resp, err := p.client.PressKey(ctx, &generated.PressKeyRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
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
// Mirror of [Page.PressKey]. Same parameter semantics; use this to
// close a press cycle that was started with PressKey.
//
// @inheritDoc [Page.PressKey]
//
// @example
//
//	_ = page.PressKey(ctx, "Shift", "ShiftLeft", 0, 1)
//	_ = page.ReleaseKey(ctx, "Shift", "ShiftLeft", 0, 1)
func (p *Page) ReleaseKey(ctx context.Context, key, code string, modifiers, location int32) error {
	resp, err := p.client.ReleaseKey(ctx, &generated.ReleaseKeyRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
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
//	sel, err := page.GetSelection(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("user selected:", sel)
func (p *Page) GetSelection(ctx context.Context) (string, error) {
	resp, err := p.client.GetSelection(ctx, &generated.GetSelectionRequest{
		SessionId: p.sessionId, PageId: p.id, ApiKey: p.apiKey,
	})
	if err != nil {
		return "", err
	}
	if e := commandErrorFrom("getSelection", resp.GetError()); e != nil {
		return "", e
	}
	return resp.Text, nil
}
