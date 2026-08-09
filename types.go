package browserscale

// Rect describes a position and size in CSS pixels.
type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// FrameInfo describes a single frame within a page's frame tree.
type FrameInfo struct {
	FrameId      string
	Url          string
	IsOOPIF      bool
	HasJSContext bool
	IsLoading    bool
	IsVisible    bool
	AbsoluteRect Rect
	RelativeRect Rect
	Children     []*FrameInfo
}

// PageInfo describes an open page (tab or popup) inside a browser context.
type PageInfo struct {
	PageId           string
	BrowserContextId string
	Url              string
	Title            string
	Viewport         Rect
	FrameTree        FrameInfo
}

// Header is a single HTTP header (name/value pair) on an intercepted
// request or response.
type Header struct {
	Name  string
	Value string
}

// InterceptedRequest describes an outgoing request captured by
// [CloudBrowser.WaitForAnyRequest].
type InterceptedRequest struct {
	Method       string
	Url          string
	Headers      []Header
	Body         string
	ResourceType string
}

// InterceptedResponse describes a network response captured by
// [CloudBrowser.WaitForAnyResponse].
type InterceptedResponse struct {
	Url        string
	StatusCode int32
	Headers    []Header
	Body       string
}

// WaitResult is the outcome of a [CloudBrowser.Wait] / [CloudBrowser.WaitForAny]
// call: which condition matched (Index, in argument order) and where the
// matched element lives.
type WaitResult struct {
	Index         int32
	FrameId       string
	BackendNodeId int32
	IsVisible     bool
	Bounds        Rect
}

// WaitConditionStatus is the per-condition diagnostic carried by [WaitError]
// when a [CloudBrowser.Wait] times out: one entry per condition (in the order
// they were passed) explaining why it never matched.
type WaitConditionStatus struct {
	// Index into the condition list this entry describes.
	Index int32
	// State is the last observed state: "not_found", "found_hidden",
	// "found_occluded" (only when the condition required visibility), or
	// "pending_steady".
	State string
	// BackendNodeId last seen for this condition (0 if never found).
	BackendNodeId int32
	// FrameId where it was last seen (empty if never found).
	FrameId string
	// IsVisible reports whether it was CSS-visible at the last observation.
	IsVisible bool
	// Bounds is the last known rect in root-viewport coordinates (nil if never
	// found).
	Bounds *Rect
	// Occluder is the intercepting element, present iff State ==
	// "found_occluded".
	Occluder *OccluderInfo
}

// WaitError is returned as the error from [CloudBrowser.Wait] when no condition
// matched before the deadline. It implements the error interface, so the
// ordinary `res, err := browser.Wait(...)` shape keeps working; recover the
// structured detail (including the per-condition breakdown) with errors.As:
//
//	res, err := browser.Wait(ctx, browserscale.CSS(".ready"))
//	var we *browserscale.WaitError
//	if errors.As(err, &we) {
//	    for _, c := range we.Conditions {
//	        log.Printf("condition %d: %s", c.Index, c.State)
//	    }
//	}
type WaitError struct {
	// Code is a machine-stable failure code, currently always "timeout".
	Code string
	// Message is a human-readable description.
	Message string
	// Conditions holds the per-condition status, same order/length as the
	// conditions passed to Wait.
	Conditions []WaitConditionStatus
}

// Error implements the error interface.
func (e *WaitError) Error() string {
	if e == nil {
		return "wait failed"
	}
	if e.Message != "" {
		return "wait failed: " + e.Code + ": " + e.Message
	}
	return "wait failed: " + e.Code
}

// NavigateResult reports where a [CloudBrowser.Navigate] call ended up
// after redirects.
type NavigateResult struct {
	FrameId string
	Url     string
}

// EvaluateResult carries the outcome of a JS evaluate call.
//
// If the expression returned a DOM element, BackendNodeId/IsVisible/Bounds
// are populated and Value is nil. Otherwise Value holds the parsed JSON
// value (string/number/bool/[]any/map[string]any/nil). On parse failure
// Value falls back to the raw server string so the caller is never empty-
// handed.
type EvaluateResult struct {
	Value         any
	BackendNodeId int32
	IsVisible     bool
	Bounds        Rect
}

// ElementResult is the outcome of an element interaction such as
// [CloudBrowser.Click], [CloudBrowser.Fill] or [CloudBrowser.ScrollTo]:
// the resolved element plus the root-relative coordinates the action
// was performed at.
type ElementResult struct {
	Success       bool
	FrameId       string
	BackendNodeId int32
	IsVisible     bool
	Bounds        Rect
	RootX         float64
	RootY         float64
}

// OccluderInfo describes the element that intercepted a click — the element
// sitting on top of the target at the intended click point. Coordinates are in
// root-viewport CSS pixels. Populated on [ClickError] for occlusion failures so
// the caller can locate and clear the blocker (e.g. find its close button).
type OccluderInfo struct {
	BackendNodeId int32
	FrameId       string
	TagName       string
	Id            string
	ClassName     string
	Text          string
	Bounds        Rect
	// PointerEvents is the blocker's computed pointer-events keyword (e.g.
	// "auto", "none", "all"). Lets you tell an invisible pass-through layer from
	// one that genuinely swallows the click.
	PointerEvents string
	// Visibility is the blocker's computed visibility keyword ("visible",
	// "hidden", "collapse").
	Visibility string
	// Opacity is the blocker's computed opacity (0..1). 0 means visually
	// invisible but it may still intercept clicks depending on PointerEvents.
	Opacity float64
	// ZIndex is the blocker's computed effective z-index as a string ("0" when
	// auto / not stacked).
	ZIndex string
	// HittableWhileInvisible is true when the blocker intercepts clicks even
	// while invisible (computed pointer-events in {all, painted, fill, stroke}):
	// a real click is swallowed even at visibility:hidden / opacity:0. When false
	// and the element is invisible, a real click would fall through.
	HittableWhileInvisible bool
	// Position is the computed position keyword. "fixed"/"sticky" means the
	// blocker is pinned (by itself or an ancestor) and stays put no matter where
	// the pointer goes — clear it by scrolling the target out from under it;
	// ordinary overlays often collapse once the pointer leaves.
	Position string
}

// ClickError is returned as the error from [CloudBrowser.Click] /
// [CloudBrowser.ClickWith] when the click did not land (the element was
// occluded and the point could not be reached). It implements the error
// interface, so the ordinary `res, err := browser.Click(...)` shape keeps
// working; recover the structured detail with errors.As:
//
//	res, err := browser.Click(ctx, browserscale.CSS("#buy"))
//	var ce *browserscale.ClickError
//	if errors.As(err, &ce) {
//	    // ce.Code, ce.Message, ce.Occluder describe the blocker
//	}
type ClickError struct {
	// Code is a machine-stable failure code, e.g. "occluded_no_reachable_point"
	// (target fully covered, no exposed part reachable) or "occluded_after_evade"
	// (a reposition was tried but the target was still covered).
	Code string
	// Message is a human-readable description.
	Message string
	// Occluder is the intercepting element (present for occlusion codes).
	Occluder *OccluderInfo
	// EvadeAttempted reports whether a pointer reposition was tried before
	// giving up.
	EvadeAttempted bool
}

// Error implements the error interface.
func (e *ClickError) Error() string {
	if e == nil {
		return "click failed"
	}
	if e.Message != "" {
		return "click failed: " + e.Code + ": " + e.Message
	}
	return "click failed: " + e.Code
}

// FillError is returned as the error from [CloudBrowser.Fill] /
// [CloudBrowser.FillWith] when the field could not be focused/typed. Fill
// focuses the field with the exact same smart click as [CloudBrowser.Click], so
// a pre-typing failure is a click failure: Code/Message mirror it and the full
// click diagnostics live under ClickError. It implements the error interface,
// so the ordinary `res, err := browser.Fill(...)` shape keeps working; recover
// the detail with errors.As:
//
//	res, err := browser.Fill(ctx, browserscale.CSS("#email"), "a@b.com")
//	var fe *browserscale.FillError
//	if errors.As(err, &fe) && fe.ClickError != nil {
//	    // fe.ClickError.Occluder describes the blocker
//	}
type FillError struct {
	// Code is the machine-stable failure code. Click-phase codes ("not_found",
	// "occluded_no_reachable_point", "occluded_after_evade") mirror the
	// underlying focus click, with diagnostics under ClickError. The focus codes
	// are "focus_stolen" (another element took focus — FocusedElement names it;
	// Fill is strictly target-bound and will not type into the thief) and
	// "focus_lost" (focus left the target and nothing is focused). For
	// untargeted stream typing that lets focus move (e.g. OTP), use Type.
	Code string
	// Message is a human-readable description (mirrors ClickError.Message).
	Message string
	// ClickError is the underlying click-core failure (locate or occlusion)
	// that prevented focusing/typing. Present for the click-phase codes; absent
	// for "focus_stolen"/"focus_lost".
	ClickError *ClickError
	// FocusedBackendNodeId is the node that held focus when Fill gave up (0 if
	// nothing was focused), for the "focus_stolen"/"focus_lost" codes.
	FocusedBackendNodeId int32
	// FocusedElement describes the element that grabbed focus instead of the
	// target ("focus_stolen"), so you can act on it (e.g. a consent button).
	FocusedElement *ElementRef
	// TargetEditable and TargetValueLength report the fill target's own state at
	// the point of failure (the focus codes): whether it is still an editable
	// text sink and its current text length. Both nil when not reported.
	TargetEditable    *bool
	TargetValueLength *int
}

// ElementRef is a lightweight descriptor of an element — enough to identify it
// (and decide what to do) without another DOM round-trip. It names the element
// that stole focus in a [FillError] focus-loss failure.
type ElementRef struct {
	// BackendNodeId is the element's stable backend node id.
	BackendNodeId int32
	// TagName is the upper-case tag name, e.g. "INPUT", "BUTTON", "DIV".
	TagName string
	// Id is the id attribute, if present.
	Id string
	// Name is the name attribute, if present.
	Name string
	// ClassName is the class attribute, if present.
	ClassName string
	// InputType is the <input> type, if the element is an <input>.
	InputType string
	// Text is a whitespace-collapsed textContent/value snippet (max 120 chars).
	Text string
	// Editable is true when the element is itself an editable text sink
	// (input / textarea / contenteditable).
	Editable bool
}

// Error implements the error interface.
func (e *FillError) Error() string {
	if e == nil {
		return "fill failed"
	}
	if e.Message != "" {
		return "fill failed: " + e.Code + ": " + e.Message
	}
	return "fill failed: " + e.Code
}

// DragError is returned as the error from [CloudBrowser.Drag] variants when the
// source element could not be acquired/pressed. Drag picks up the source with
// the same smart click as [CloudBrowser.Click], so a pre-drag failure is a
// click failure: Code/Message mirror it and the full click diagnostics live
// under ClickError. Implements the error interface; recover with errors.As.
type DragError struct {
	// Code is mirrored from the underlying click failure: "not_found",
	// "occluded_no_reachable_point" or "occluded_after_evade".
	Code string
	// Message is a human-readable description (mirrors ClickError.Message).
	Message string
	// ClickError is the underlying click-core failure at the source pickup.
	ClickError *ClickError
}

// Error implements the error interface.
func (e *DragError) Error() string {
	if e == nil {
		return "drag failed"
	}
	if e.Message != "" {
		return "drag failed: " + e.Code + ": " + e.Message
	}
	return "drag failed: " + e.Code
}

// SelectOptionError is returned as the error from [CloudBrowser] SelectByXxx
// calls when the option could not be selected. selectOption is programmatic (no
// pointer gate), so it only reports semantic failures. Implements the error
// interface; recover with errors.As.
type SelectOptionError struct {
	// Code is "not_found" (the <select> was not located) or "option_not_found"
	// (no option matched the requested index/value/text).
	Code string
	// Message is a human-readable description.
	Message string
}

// Error implements the error interface.
func (e *SelectOptionError) Error() string {
	if e == nil {
		return "selectOption failed"
	}
	if e.Message != "" {
		return "selectOption failed: " + e.Code + ": " + e.Message
	}
	return "selectOption failed: " + e.Code
}

// ScrollError is returned as the error from [CloudBrowser.ScrollTo] when the
// target could not be located/scrolled. Implements the error interface; recover
// with errors.As.
type ScrollError struct {
	// Code is currently always "not_found".
	Code string
	// Message is a human-readable description.
	Message string
}

// Error implements the error interface.
func (e *ScrollError) Error() string {
	if e == nil {
		return "scrollTo failed"
	}
	if e.Message != "" {
		return "scrollTo failed: " + e.Code + ": " + e.Message
	}
	return "scrollTo failed: " + e.Code
}

// MoveError is returned as the error from [CloudBrowser.MoveTo] when the target
// could not be located. A move has no occlusion notion, so this is the only
// semantic failure. Implements the error interface; recover with errors.As.
type MoveError struct {
	// Code is currently always "not_found".
	Code string
	// Message is a human-readable description.
	Message string
}

// Error implements the error interface.
func (e *MoveError) Error() string {
	if e == nil {
		return "moveTo failed"
	}
	if e.Message != "" {
		return "moveTo failed: " + e.Code + ": " + e.Message
	}
	return "moveTo failed: " + e.Code
}

// DragResult is the outcome of a [CloudBrowser.Drag] gesture: the resolved
// source element and the start/end coordinates of the performed drag.
type DragResult struct {
	Success       bool
	FrameId       string
	BackendNodeId int32
	StartX        float64
	StartY        float64
	EndX          float64
	EndY          float64
}

// SelectOptionResult reports which <option> a SelectByXxx call ended up
// selecting.
type SelectOptionResult struct {
	Success       bool
	SelectedIndex int32
	SelectedValue string
	SelectedText  string
}

// ObservationOpts customizes a [CloudBrowser.GetObservationWith] call.
// Zero/empty values mean "use the server default".
type ObservationOpts struct {
	// Format is "text" (default) for the compact line format meant to be handed
	// to a model as-is, or "json" for the structured form. Only the requested
	// representation is built, so asking for one does not cost the other.
	Format string

	// MaxElementsPerFrame caps emitted elements per frame. 0 = server default
	// (800). This is a safety net against runaway documents; MaxTotalTokens is
	// the limit that normally binds.
	MaxElementsPerFrame int32

	// MaxTextLength caps human-readable strings (labels, text, values) in
	// characters. 0 = server default (300). Identifier-like attributes (type,
	// name, role) have their own fixed, shorter cap and are unaffected.
	MaxTextLength int32

	// MaxTotalTokens budgets the whole page in estimated tokens rather than
	// characters, because the same character count is worth roughly four times
	// as many tokens in CJK text as in ASCII. 0 = server default (8000). Frames
	// are visited in tree order and each gets whatever is left.
	MaxTotalTokens int32

	// IncludeBounds adds bounds="x,y,w,h" to every row. Off by default; bounds
	// cost about as much as the rest of a row and are rarely needed, since
	// elements are addressed by backendNodeId.
	IncludeBounds bool

	// ViewportOnly limits the walk to elements intersecting the frame's current
	// viewport. Off by default.
	ViewportOnly bool

	// Subtree scope — set exactly one of BackendNodeId, Selector or
	// JSExpression to observe only that element's subtree (follow-up looks at a
	// form then cost the form, not the ads around it). Omit all three for the
	// whole page. Child iframes reached inside the scope are still visited.
	BackendNodeId int32
	Selector      string
	JSExpression  string
	// InFrame looks up the scope root: empty = main frame, a frameId, or
	// [AllFrames]. Ignored when observing the whole page.
	InFrame string
}

// ScreenshotResult is a single captured image of the page, returned by
// [CloudBrowser.Screenshot]. DataBase64 holds the encoded image bytes
// (PNG by default); Width and Height are in physical pixels.
type ScreenshotResult struct {
	DataBase64 string
	Width      int32
	Height     int32
}

// ReadCanvasResult is the pixel readback of a <canvas>, returned by
// [CloudBrowser.ReadCanvas]. DataBase64 holds the encoded image bytes (PNG by
// default) or the raw RGBA buffer when Opts.Format == "rgba". OriginClean
// reports whether the canvas was untainted (informational; the read succeeds
// either way).
type ReadCanvasResult struct {
	Success       bool
	FrameId       string
	BackendNodeId int32
	DataBase64    string
	Width         int32
	Height        int32
	OriginClean   bool
}

// InspectResult describes the topmost element hit at viewport-relative
// (x, y). BackendNodeId == 0 means nothing was found at that position.
type InspectResult struct {
	BackendNodeId int32
	FrameId       string
	TagName       string
	TextContent   string
	IsVisible     bool
	Bounds        Rect
}
