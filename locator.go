package browserscale

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/browserscale/browserscale-go/generated"
)

// Locator says what a call is about: an element, and where on the page to look
// for it. It is used as a wait condition (passed to [Page.Wait]), as the
// target of an action (passed to [Page.Click], [Page.Fill], ...), and as a
// container that another locator is looked for in (passed to [Locator.In]).
//
// Build one with a constructor, never by hand:
//
//   - [CSS], [JS]: an element by selector or by expression
//   - a result's Ref (or [Ref] for a reference read from an observation): the
//     very element an earlier call found
//   - [At]: a point instead of an element, for [Page.Click] and [Page.MoveTo]
//   - [FrameURL], [FrameName], [MainFrame], [AnyFrame]: a frame, to look in
//
// Where a locator looks is decided by [Locator.In]. Without it a [CSS]
// locator means the whole page, every frame included, and a [JS] locator runs
// in the page's main document.
//
// Not every part is meaningful everywhere: Visible and Steady apply to wait
// conditions and reactions, [At] to Click and MoveTo, and the frame
// constructors only name a place to look in, never a target.
type Locator struct {
	selector     string
	jsExpression string
	ref          string

	// A frame, named directly. These only work as a container.
	frameURL  string
	frameName *string
	frameID   string
	mainFrame bool
	anyFrame  bool

	// The element has to hold a document of its own; see Frame.
	mustBeFrame bool

	// What this locator is looked for in; nil for the default.
	in *Locator

	// The frame a reference was seen in, when it came out of a result. Lets a
	// stale reference say where that frame is now.
	seenIn string

	visible    *bool
	steadyTime *float64

	x *float64
	y *float64
}

// CSS waits for / targets an element matching the given CSS selector.
//
// Without [Locator.In] the selector is looked for on the whole page: the main
// document first, then every frame in the order they appear, frames nobody
// can see last. The result says which frame it was found in, so a button
// inside a consent or payment iframe needs no frame named. Use In to confine
// the search to one frame, one dialog or one component.
//
// Where the element has to be visible (a wait condition or reaction, and every
// action), the selector means its first rendered match: matches that are not
// rendered at all (display:none, visibility:hidden, no size) are passed over,
// so a hidden copy such as the one in a closed mobile menu does not shadow the
// visible element. A covered or offscreen match is still rendered and stays
// the target. When a later match was taken, the result says which one in
// SelectorMatch. With Visible(false) a wait or reaction takes the first match.
//
// When used in [Page.Wait], the condition requires the element to be
// visible and to hold still for 500ms before it matches — the API's defaults
// for a condition that does not set them. Override per call with
// [Locator.Visible] / [Locator.Steady] (use `.Steady(0)` to disable the steady
// check).
//
// When used as an action target (Click, etc.) the visible/steady fields
// are ignored — there are no corresponding fields on the action requests.
//
// @param selector - CSS selector matching the element
//
// @returns *Locator usable as a wait condition, an action target or a container
//
// @example
//
//	// As a wait condition.
//	_, _ = page.Wait(ctx, browserscale.CSS("button.submit"))
//	// As an action target. Found wherever it is, also inside an iframe.
//	res, _ := page.Click(ctx, browserscale.CSS("button.accept"))
//	fmt.Println(res.Frame.Main, res.Frame.URL)
func CSS(selector string) *Locator {
	return &Locator{selector: selector}
}

// JS waits for / targets the result of a JavaScript expression.
//
// Without [Locator.In] the expression runs in the page's main document, and
// only there. With In it runs in the document the container leads to; when
// the container is an element rather than a frame, the expression gets that
// element as __wrc.scope and has to return an element inside it.
//
// Same wait defaults as [CSS] (visible, 500ms steady); these only apply when
// the expression returns a DOM Element. For non-Element truthy values (boolean,
// string, number, plain object) both are no-ops and the condition matches as
// soon as the value is truthy.
//
// Use [Locator.Visible](false) / [Locator.Steady](0) on the returned Locator
// to opt out.
//
// @param expression - JavaScript expression
//
// @returns *Locator usable as a wait condition, an action target or a container
//
// @example
//
//	_, _ = page.Wait(ctx, browserscale.JS("window.__ready === true"))
//
// @example
//
//	// By text, inside one frame.
//	pay := browserscale.FrameURL("*checkout.example.com/*")
//	_, _ = page.Click(ctx, browserscale.JS(
//	    `[...document.querySelectorAll('button')].find(b => /Pay/.test(b.innerText))`).In(pay))
func JS(expression string) *Locator {
	return &Locator{jsExpression: expression}
}

// Ref targets the very element an earlier call found, by its reference.
//
// Every result that names an element carries its reference as a ready
// Locator (WaitElement.Ref, ElementResult.Ref, OccluderInfo.Ref, ...), so this
// constructor is only needed for a reference that arrived as text: an
// observation prints one per frame ("# ref @<document>"), and a row's number
// followed by that suffix is the element's reference.
//
// A reference names its element and the document it lives in, so no frame is
// named with it and [Locator.In] does not apply. It stays good while the
// element is in its document. Once a navigation replaced the document, or the
// element was removed, a call given it fails with a [*StaleRefError] instead
// of acting on whatever took its place.
//
// @param value - the reference, as [Locator.RefString] returns it
//
// @returns *Locator usable as an action target or a container
//
// @example
//
//	res, _ := page.Wait(ctx, browserscale.CSS(".result"))
//	_, err := page.Click(ctx, res.Element.Ref)
//
// @example
//
//	// From an observation: row 47 of the frame whose header says "# ref @9F2C...".
//	_, err := page.Click(ctx, browserscale.Ref("47@9F2C41D0A1B24C7E8F3A5D6B7C8E9F01"))
func Ref(value string) *Locator {
	return &Locator{ref: value}
}

// At targets viewport coordinates instead of an element.
//
// Useful for clicking inside a canvas, hovering decorative regions, or
// dispatching events at synthetic positions. Action-only — using it in
// [Page.Wait] returns an error at send time. Note that only Click
// and MoveTo accept At; Scroll, Drag, Fill and Select all require a real
// element.
//
// @param x - viewport-relative x in CSS pixels
// @param y - viewport-relative y in CSS pixels
//
// @returns *Locator usable only as an action target
//
// @example
//
//	// Click at canvas-relative coordinates.
//	_, _ = page.Click(ctx, browserscale.At(120, 240))
func At(x, y float64) *Locator {
	return &Locator{x: &x, y: &y}
}

// ── Frames, as places to look in ──

// FrameURL names a frame by the URL of the document it shows.
//
// It is a container: pass it to [Locator.In], [Page.EvaluateIn] or
// [ObservationOpts].Scope. The frame is looked for at any depth below the
// page, or below the container it is itself put in with In, and a frame a
// person can see is preferred over one that is hidden or has no size. Use
// this when the iframe element has nothing stable to select it by.
//
// @param pattern - URL wildcard; `*` matches any character span
//
// @returns *Locator usable only as a container
//
// @example
//
//	challenge := browserscale.FrameURL("*hcaptcha.com/*frame=checkbox*")
//	_, _ = page.Click(ctx, browserscale.CSS("#checkbox").In(challenge))
//
// @example
//
//	// A frame inside a frame.
//	outer := browserscale.FrameURL("*/_Incapsula_Resource*")
//	inner := browserscale.FrameURL("*hcaptcha.com/*frame=checkbox*").In(outer)
//	_, _ = page.Click(ctx, browserscale.CSS("#checkbox").In(inner))
func FrameURL(pattern string) *Locator {
	return &Locator{frameURL: pattern}
}

// FrameName names a frame by its name: window.name, which starts out as the
// iframe's name attribute. Otherwise as [FrameURL].
//
// @param name - the frame's name
//
// @returns *Locator usable only as a container
//
// @example
//
//	_, _ = page.Fill(ctx, browserscale.CSS("input[name=q]").In(browserscale.FrameName("search")), "shoes")
func FrameName(name string) *Locator {
	return &Locator{frameName: &name}
}

// FrameByID names one frame by its id, as a result's Frame.ID or
// [Page.Frames] report it. The id stays the same across navigations of the
// frame, but belongs to this session: prefer [FrameURL], [FrameName] or the
// iframe's own selector in anything that is written down.
//
// @param frameID - the frame's id
//
// @returns *Locator usable only as a container
//
// @example
//
//	res, _ := page.Wait(ctx, browserscale.CSS("#card"))
//	frame := browserscale.FrameByID(res.Element.Frame.ID)
//	_, _ = page.EvaluateIn(ctx, frame, "document.title")
func FrameByID(frameID string) *Locator {
	return &Locator{frameID: frameID}
}

// MainFrame names the page's main document. A [CSS] locator put in it is not
// looked for in any iframe.
//
// @returns *Locator usable only as a container
//
// @example
//
//	// Only the page's own button, not one in an embedded widget.
//	_, _ = page.Click(ctx, browserscale.CSS("button.accept").In(browserscale.MainFrame()))
func MainFrame() *Locator {
	return &Locator{mainFrame: true}
}

// AnyFrame names every frame, one after the other: the main document, then
// each iframe. It is what makes a [JS] locator search all frames, which it
// does not do by itself. Put in a frame container with In, it means that
// frame and the frames below it.
//
// @returns *Locator usable only as a container
//
// @example
//
//	_, _ = page.Wait(ctx, browserscale.JS(
//	    `[...document.querySelectorAll('button')].find(b => b.innerText === 'Accept all')`,
//	).In(browserscale.AnyFrame()))
func AnyFrame() *Locator {
	return &Locator{anyFrame: true}
}

// Frame requires the element a locator names to be an iframe.
//
// A container made with [CSS], [JS] or a reference is entered as a frame when
// the element is an iframe and searched as a subtree when it is anything else,
// which is usually what is wanted. Wrap it in Frame to hear about it when the
// page gave something else: the call then fails with code
// "container_not_found", saying the element is not a frame, instead of
// quietly searching a subtree.
//
// @param container - a [CSS], [JS] or reference locator
//
// @returns *Locator usable as a container
//
// @example
//
//	pay := browserscale.Frame(browserscale.CSS("iframe.pay"))
//	_, _ = page.Fill(ctx, browserscale.CSS("input[name=cardnumber]").In(pay), card)
func Frame(container *Locator) *Locator {
	if container == nil {
		return nil
	}
	c := container.clone()
	c.mustBeFrame = true
	return c
}

// ── Locator modifiers (per-locator) ──

// In says where this locator is looked for: inside the container.
//
// The container is another locator, and what it means depends on what it
// finds:
//
//   - an iframe ([CSS], [JS] or a reference naming the iframe element): its
//     document, same-origin or not
//   - any other element: its subtree. For a component with a shadow tree that
//     includes the shadow tree, open or closed, so a host element is the way
//     into it
//   - [FrameURL], [FrameName], [FrameByID], [MainFrame], [AnyFrame]: that frame
//
// A container can itself be put in a container, to any depth. Calling In again
// on the result wraps one level further out, so `a.In(b).In(c)` is a in b in c.
//
// The whole chain is looked up anew each time the call looks for its element.
// A call that waits ([Page.Click], [Page.Fill], [Page.Wait], a reaction)
// therefore also waits for the iframe and the container to appear, and a
// container that never does is reported as "container_not_found" with the
// link that was missing.
//
// In returns a new Locator and leaves both this one and the container as they
// were, so a container can be kept and reused.
//
// @param container - where to look
//
// @returns a new *Locator for the same element, confined to the container
//
// @example
//
//	pay := browserscale.CSS("iframe[title='Secure payment']")
//	_, _ = page.Fill(ctx, browserscale.CSS("input[name=cardnumber]").In(pay), card)
//	_, _ = page.Fill(ctx, browserscale.CSS("input[name=cvc]").In(pay), cvc)
//
// @example
//
//	// A subtree, and a shadow host.
//	dialog := browserscale.CSS("[role=dialog]")
//	_, _ = page.Click(ctx, browserscale.CSS("button.close").In(dialog))
//	picker := browserscale.CSS("date-picker")
//	_, _ = page.Click(ctx, browserscale.CSS("button.next").In(picker))
//
// @example
//
//	// The iframe a wait found, as the place to look next.
//	frame, _ := page.Wait(ctx, browserscale.CSS("iframe.widget"))
//	_, _ = page.Click(ctx, browserscale.CSS("button").In(frame.Element.Ref))
func (l *Locator) In(container *Locator) *Locator {
	if l == nil {
		return nil
	}
	out := *l
	out.in = wrapIn(l.in, container)
	return &out
}

// wrapIn returns a copy of the chain `in` with container put around its
// outermost link. Neither argument is changed.
func wrapIn(in, container *Locator) *Locator {
	if in == nil {
		return container.clone()
	}
	out := *in
	out.in = wrapIn(in.in, container)
	return &out
}

// Visible enforces or disables the visibility check for this Locator's
// wait condition.
//
// Visibility is required by default, so pass false to wait for DOM presence
// alone. Has no effect when the Locator is used as an action target — actions
// never check visibility before dispatching.
//
// @param v - true to require visibility, false to skip the check
//
// @returns the same Locator for chaining
//
// @example
//
//	_, _ = page.Wait(ctx, browserscale.CSS("#hidden").Visible(false))
func (l *Locator) Visible(v bool) *Locator { l.visible = &v; return l }

// Steady requires the element to keep a stable position and size for at
// least ms milliseconds before the wait matches.
//
// Settling defaults to 500ms, so pass 0 to match the instant the element is
// found. Has no effect for JS expressions that return a non-Element value, nor
// when the Locator is used as an action target.
//
// @param ms - steady-state duration in milliseconds; 0 disables
//
// @returns the same Locator for chaining
//
// @example
//
//	_, _ = page.Wait(ctx, browserscale.CSS(".banner").Steady(0))
func (l *Locator) Steady(ms float64) *Locator { l.steadyTime = &ms; return l }

// RefString returns the reference this Locator stands for, in the form [Ref]
// takes, or "" when it is not a reference. For writing a reference down or
// handing it to something that is not Go code.
//
// @returns the reference as text, or ""
//
// @example
//
//	res, _ := page.Wait(ctx, browserscale.CSS(".result"))
//	saved := res.Element.Ref.RefString()
//	// ... later, in the same session:
//	_, _ = page.Click(ctx, browserscale.Ref(saved))
func (l *Locator) RefString() string {
	if l == nil {
		return ""
	}
	return l.ref
}

// String describes the locator for logs and error messages: what it names and
// what it is looked for in, outermost last.
func (l *Locator) String() string {
	if l == nil {
		return "<nil>"
	}
	var b strings.Builder
	for cur := l; cur != nil; cur = cur.in {
		if cur != l {
			b.WriteString(" in ")
		}
		b.WriteString(cur.describe())
	}
	return b.String()
}

// MarshalJSON writes the locator as text, so that a result printed as JSON
// shows what its Ref is: a reference as [Locator.RefString] gives it, ready
// for [Ref], and any other locator as [Locator.String] describes it.
func (l *Locator) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("null"), nil
	}
	if l.ref != "" && l.in == nil {
		return json.Marshal(l.ref)
	}
	return json.Marshal(l.String())
}

func (l *Locator) describe() string {
	var s string
	switch {
	case l.selector != "":
		s = fmt.Sprintf("css %q", l.selector)
	case l.jsExpression != "":
		s = fmt.Sprintf("js %q", l.jsExpression)
	case l.ref != "":
		s = "ref " + l.ref
	case l.frameURL != "":
		s = fmt.Sprintf("frame url %q", l.frameURL)
	case l.frameName != nil:
		s = fmt.Sprintf("frame name %q", *l.frameName)
	case l.frameID != "":
		s = "frame " + l.frameID
	case l.mainFrame:
		s = "main frame"
	case l.anyFrame:
		s = "any frame"
	case l.x != nil && l.y != nil:
		s = fmt.Sprintf("at (%g, %g)", *l.x, *l.y)
	default:
		s = "empty locator"
	}
	if l.mustBeFrame {
		s = "frame(" + s + ")"
	}
	return s
}

// ── internals ──

// clone copies the locator and the chain it is looked for in, so that the copy
// shares nothing that a later In could change.
func (l *Locator) clone() *Locator {
	if l == nil {
		return nil
	}
	c := *l
	c.in = l.in.clone()
	return &c
}

// isElement reports whether the locator names an element (rather than a frame
// or a point).
func (l *Locator) isElement() bool {
	return l.selector != "" || l.jsExpression != "" || l.ref != ""
}

// isFrame reports whether the locator names a frame directly.
func (l *Locator) isFrame() bool {
	return l.frameURL != "" || l.frameName != nil || l.frameID != "" || l.mainFrame || l.anyFrame
}

// validateTarget validates the locator for an element-action call.
// allowCoords controls whether an At(x,y) locator is acceptable for this
// command (Click and MoveTo accept it; ScrollTo, Drag, SelectOption, Fill
// require an actual element).
func (l *Locator) validateTarget(cmd string, allowCoords bool) error {
	if l == nil {
		return fmt.Errorf("browserscale.%s: a target locator is required", cmd)
	}
	hasCoords := l.x != nil && l.y != nil
	switch {
	case l.isFrame():
		return fmt.Errorf("browserscale.%s: %s names a frame, not an element; put the element's locator in it with In", cmd, l.describe())
	case !l.isElement() && !hasCoords:
		return fmt.Errorf("browserscale.%s: target must be a CSS, JS or reference locator, or At(x,y)", cmd)
	case hasCoords && !allowCoords:
		return fmt.Errorf("browserscale.%s: At(x,y) is not supported here — use an element locator", cmd)
	case hasCoords && l.in != nil:
		return fmt.Errorf("browserscale.%s: At(x,y) is a point of the viewport and cannot be put in a container", cmd)
	}
	return l.validateChain(cmd)
}

// validateChain checks the containers the locator is looked for in.
func (l *Locator) validateChain(cmd string) error {
	if l.ref != "" && l.in != nil {
		return fmt.Errorf("browserscale.%s: a reference names its element and frame by itself; In does not apply to it", cmd)
	}
	for c := l.in; c != nil; c = c.in {
		if err := c.validateContainer(cmd); err != nil {
			return err
		}
	}
	return nil
}

// validateContainer checks that the locator can serve as a place to look in.
func (l *Locator) validateContainer(cmd string) error {
	switch {
	case l == nil:
		return fmt.Errorf("browserscale.%s: a container locator is required", cmd)
	case l.x != nil || l.y != nil:
		return fmt.Errorf("browserscale.%s: At(x,y) cannot be a container", cmd)
	case !l.isElement() && !l.isFrame():
		return fmt.Errorf("browserscale.%s: an empty locator cannot be a container", cmd)
	case l.ref != "" && l.in != nil:
		return fmt.Errorf("browserscale.%s: a reference names its element and frame by itself; In does not apply to it", cmd)
	}
	return nil
}

// link is this locator as one step of a chain.
func (l *Locator) link() *generated.Container {
	c := &generated.Container{Frame: l.mustBeFrame}
	switch {
	case l.selector != "":
		c.Link = &generated.Container_Selector{Selector: l.selector}
	case l.jsExpression != "":
		c.Link = &generated.Container_JsExpression{JsExpression: l.jsExpression}
	case l.ref != "":
		c.Link = &generated.Container_Ref{Ref: l.ref}
	case l.frameURL != "":
		c.Link = &generated.Container_FrameUrl{FrameUrl: l.frameURL}
	case l.frameName != nil:
		c.Link = &generated.Container_FrameName{FrameName: *l.frameName}
	case l.frameID != "":
		c.Link = &generated.Container_FrameId{FrameId: l.frameID}
	case l.mainFrame:
		c.Link = &generated.Container_Main{Main: true}
	case l.anyFrame:
		c.Link = &generated.Container_Any{Any: true}
	}
	return c
}

// chain is where this locator is looked for, as the wire wants it: outermost
// first, this locator itself not included.
func (l *Locator) chain() []*generated.Container {
	if l == nil || l.in == nil {
		return nil
	}
	return l.in.chainWithSelf()
}

// chainWithSelf is the chain that leads to this locator and into it: what a
// call uses when the locator is itself the place to act in.
func (l *Locator) chainWithSelf() []*generated.Container {
	if l == nil {
		return nil
	}
	return append(l.in.chainWithSelf(), l.link())
}

// locatorFromChain rebuilds the container a chain describes (outermost
// first), the inverse of chainWithSelf. Nil for an empty chain.
func locatorFromChain(chain []*generated.Container) *Locator {
	var outer *Locator
	for _, c := range chain {
		l := &Locator{mustBeFrame: c.GetFrame(), in: outer}
		switch link := c.GetLink().(type) {
		case *generated.Container_Selector:
			l.selector = link.Selector
		case *generated.Container_JsExpression:
			l.jsExpression = link.JsExpression
		case *generated.Container_Ref:
			l.ref = link.Ref
		case *generated.Container_FrameUrl:
			l.frameURL = link.FrameUrl
		case *generated.Container_FrameName:
			name := link.FrameName
			l.frameName = &name
		case *generated.Container_FrameId:
			l.frameID = link.FrameId
		case *generated.Container_Main:
			l.mainFrame = true
		case *generated.Container_Any:
			l.anyFrame = true
		}
		outer = l
	}
	return outer
}

// refLocator is the Locator a result hands out for the element it found, or
// nil when the result names none (a click at coordinates, an expression that
// answered with a value).
func refLocator(ref string, frame *generated.FrameRef) *Locator {
	return refLocatorIn(ref, frame.GetFrameId())
}

// refLocatorIn is refLocator for a result that names the frame by id only.
func refLocatorIn(ref, frameID string) *Locator {
	if ref == "" {
		return nil
	}
	return &Locator{ref: ref, seenIn: frameID}
}
