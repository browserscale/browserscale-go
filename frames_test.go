package browserscale

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/browserscale/browserscale-go/generated"

	"google.golang.org/grpc"
)

// fakeBrowser answers the calls these tests make and keeps the request each
// one was given. Any other call panics on the nil embedded client, which is
// the point: a test that reaches for something unexpected should say so.
type fakeBrowser struct {
	generated.BrowserClient

	click    *generated.ClickRequest
	fill     *generated.FillRequest
	wait     *generated.WaitForAnyParams
	evaluate *generated.EvaluateRequest
	observe  *generated.GetObservationRequest
	reaction *generated.AddReactionRequest
	hl       *generated.HighlightNodeRequest
	nodeRef  *generated.GetNodeRefRequest

	clickResult    *generated.ClickResult
	fillResult     *generated.FillResult
	waitResult     *generated.WaitResult
	evalResult     *generated.EvaluateResponse
	observeResult  *generated.GetObservationResponse
	hlResult       *generated.CommandResult
	pages          []*generated.PageInfo
	reactions      []*generated.ReactionInfo
	focusedPage    string
	getPagesCalled int
}

func (f *fakeBrowser) Click(_ context.Context, in *generated.ClickRequest, _ ...grpc.CallOption) (*generated.ClickResult, error) {
	f.click = in
	if f.clickResult != nil {
		return f.clickResult, nil
	}
	return &generated.ClickResult{Success: true}, nil
}

func (f *fakeBrowser) Fill(_ context.Context, in *generated.FillRequest, _ ...grpc.CallOption) (*generated.FillResult, error) {
	f.fill = in
	if f.fillResult != nil {
		return f.fillResult, nil
	}
	return &generated.FillResult{Success: true}, nil
}

func (f *fakeBrowser) WaitForAny(_ context.Context, in *generated.WaitForAnyParams, _ ...grpc.CallOption) (*generated.WaitResult, error) {
	f.wait = in
	if f.waitResult != nil {
		return f.waitResult, nil
	}
	return &generated.WaitResult{Success: true, Kind: "element", Element: &generated.WaitElementMatch{}}, nil
}

func (f *fakeBrowser) Evaluate(_ context.Context, in *generated.EvaluateRequest, _ ...grpc.CallOption) (*generated.EvaluateResponse, error) {
	f.evaluate = in
	if f.evalResult != nil {
		return f.evalResult, nil
	}
	return &generated.EvaluateResponse{Success: true, Result: `"ok"`}, nil
}

func (f *fakeBrowser) GetObservation(_ context.Context, in *generated.GetObservationRequest, _ ...grpc.CallOption) (*generated.GetObservationResponse, error) {
	f.observe = in
	if f.observeResult != nil {
		return f.observeResult, nil
	}
	return &generated.GetObservationResponse{Success: true, Observation: "# frame"}, nil
}

func (f *fakeBrowser) AddReaction(_ context.Context, in *generated.AddReactionRequest, _ ...grpc.CallOption) (*generated.AddReactionResponse, error) {
	f.reaction = in
	return &generated.AddReactionResponse{ReactionId: "r1", Success: true}, nil
}

func (f *fakeBrowser) ListReactions(_ context.Context, _ *generated.ListReactionsRequest, _ ...grpc.CallOption) (*generated.ListReactionsResponse, error) {
	return &generated.ListReactionsResponse{Success: true, Reactions: f.reactions}, nil
}

func (f *fakeBrowser) HighlightNode(_ context.Context, in *generated.HighlightNodeRequest, _ ...grpc.CallOption) (*generated.CommandResult, error) {
	f.hl = in
	if f.hlResult != nil {
		return f.hlResult, nil
	}
	return &generated.CommandResult{Success: true}, nil
}

func (f *fakeBrowser) GetNodeRef(_ context.Context, in *generated.GetNodeRefRequest, _ ...grpc.CallOption) (*generated.GetNodeRefResponse, error) {
	f.nodeRef = in
	return &generated.GetNodeRefResponse{
		Success: true,
		Ref:     "12@" + docA,
		Frame:   &generated.FrameRef{FrameId: in.GetFrameId(), Url: "https://pay.example/"},
	}, nil
}

func (f *fakeBrowser) GetPages(_ context.Context, _ *generated.GetPagesRequest, _ ...grpc.CallOption) (*generated.GetPagesResponse, error) {
	f.getPagesCalled++
	return &generated.GetPagesResponse{Success: true, Pages: f.pages}, nil
}

func (f *fakeBrowser) GetFocusedPage(_ context.Context, _ *generated.GetFocusedPageRequest, _ ...grpc.CallOption) (*generated.GetFocusedPageResponse, error) {
	return &generated.GetFocusedPageResponse{PageId: f.focusedPage}, nil
}

const docA = "0123456789ABCDEF0123456789ABCDEF"

func newFakePage() (*Page, *fakeBrowser) {
	fb := &fakeBrowser{}
	b := &CloudBrowser{client: fb, sessionId: "s", apiKey: "k"}
	return b.GetPage("p1"), fb
}

// describeChain renders a wire chain compactly: "url:*a* > css:#b > frame(css:#c)".
func describeChain(chain []*generated.Container) string {
	parts := make([]string, len(chain))
	for i, c := range chain {
		var s string
		switch link := c.GetLink().(type) {
		case *generated.Container_Selector:
			s = "css:" + link.Selector
		case *generated.Container_JsExpression:
			s = "js:" + link.JsExpression
		case *generated.Container_Ref:
			s = "ref:" + link.Ref
		case *generated.Container_FrameUrl:
			s = "url:" + link.FrameUrl
		case *generated.Container_FrameName:
			s = "name:" + link.FrameName
		case *generated.Container_FrameId:
			s = "id:" + link.FrameId
		case *generated.Container_Main:
			s = "main"
		case *generated.Container_Any:
			s = "any"
		default:
			s = "?"
		}
		if c.GetFrame() {
			s = "frame(" + s + ")"
		}
		parts[i] = s
	}
	return strings.Join(parts, " > ")
}

func TestChainIsSentOutermostFirst(t *testing.T) {
	imperva := FrameURL("*/_Incapsula_Resource*")
	hcaptcha := FrameURL("*hcaptcha.com/*frame=checkbox*").In(imperva)
	target := CSS("#checkbox").In(hcaptcha)

	if got, want := describeChain(target.chain()), "url:*/_Incapsula_Resource* > url:*hcaptcha.com/*frame=checkbox*"; got != want {
		t.Fatalf("chain = %q, want %q", got, want)
	}
	// Calling In on a locator that already has a container wraps further out.
	stacked := CSS("button").In(CSS("[role=dialog]")).In(FrameName("app"))
	if got, want := describeChain(stacked.chain()), "name:app > css:[role=dialog]"; got != want {
		t.Fatalf("stacked chain = %q, want %q", got, want)
	}
}

func TestInLeavesBothLocatorsReusable(t *testing.T) {
	pay := CSS("iframe[title='Secure payment']")
	card := CSS("input[name=cardnumber]")

	number := card.In(pay)
	cvc := CSS("input[name=cvc]").In(pay)
	deeper := number.In(MainFrame())

	if pay.in != nil || card.in != nil {
		t.Fatal("In changed the locators it was called with")
	}
	if got := describeChain(number.chain()); got != "css:iframe[title='Secure payment']" {
		t.Fatalf("number chain changed by a later In: %q", got)
	}
	if got := describeChain(cvc.chain()); got != "css:iframe[title='Secure payment']" {
		t.Fatalf("cvc chain = %q", got)
	}
	if got := describeChain(deeper.chain()); got != "main > css:iframe[title='Secure payment']" {
		t.Fatalf("deeper chain = %q", got)
	}
}

func TestFrameAssertionMarksTheLink(t *testing.T) {
	plain := CSS("iframe.pay")
	strict := Frame(plain)
	if plain.mustBeFrame {
		t.Fatal("Frame changed the locator it was given")
	}
	if got := describeChain(CSS("#card").In(strict).chain()); got != "frame(css:iframe.pay)" {
		t.Fatalf("chain = %q", got)
	}
	if got := describeChain(CSS("#card").In(plain).chain()); got != "css:iframe.pay" {
		t.Fatalf("chain = %q", got)
	}
}

func TestEveryContainerKindReachesTheWire(t *testing.T) {
	chain := CSS("#x").
		In(CSS("#host")).
		In(JS("document.body")).
		In(Ref("7@" + docA)).
		In(FrameByID("F1")).
		In(FrameName("")).
		In(FrameURL("*a*")).
		In(AnyFrame()).
		In(MainFrame()).chain()
	want := "main > any > url:*a* > name: > id:F1 > ref:7@" + docA + " > js:document.body > css:#host"
	if got := describeChain(chain); got != want {
		t.Fatalf("chain = %q\nwant    %q", got, want)
	}
	// And back: what the server lists is the same container again.
	if got := describeChain(locatorFromChain(chain).chainWithSelf()); got != want {
		t.Fatalf("round trip = %q\nwant        %q", got, want)
	}
}

func TestTargetValidation(t *testing.T) {
	page, fb := newFakePage()
	ctx := context.Background()
	cases := []struct {
		name   string
		target *Locator
		want   string
	}{
		{"nil", nil, "required"},
		{"a frame is not a target", FrameURL("*x*"), "names a frame"},
		{"main frame is not a target", MainFrame(), "names a frame"},
		{"a point has no container", At(1, 2).In(MainFrame()), "cannot be put in a container"},
		{"a reference has no container", Ref("7@" + docA).In(MainFrame()), "In does not apply"},
		{"a point is no container", CSS("a").In(At(1, 2)), "cannot be a container"},
		{"a reference in a chain has no container", CSS("a").In(Ref("7@" + docA).In(MainFrame())), "In does not apply"},
	}
	for _, c := range cases {
		_, err := page.Click(ctx, c.target)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to mention %q", c.name, err, c.want)
		}
	}
	if fb.click != nil {
		t.Fatal("an invalid target was sent")
	}
	// Fill takes no point.
	if _, err := page.Fill(ctx, At(1, 2), "x"); err == nil {
		t.Fatal("Fill accepted At")
	}
}

func TestClickSendsTargetAndReportsWhereItWasFound(t *testing.T) {
	page, fb := newFakePage()
	ctx := context.Background()
	fb.clickResult = &generated.ClickResult{
		Success: true,
		Ref:     "41@" + docA,
		Frame:   &generated.FrameRef{FrameId: "F1", Url: "https://consent.example/", Main: false},
	}

	res, err := page.Click(ctx, CSS("button.accept"))
	if err != nil {
		t.Fatal(err)
	}
	if fb.click.GetSelector() != "button.accept" || fb.click.Ref != nil || fb.click.JsExpression != nil || len(fb.click.In) != 0 {
		t.Fatalf("request = %+v", fb.click)
	}
	if res.Frame.Main || res.Frame.URL != "https://consent.example/" || res.Frame.ID != "F1" {
		t.Fatalf("frame = %+v", res.Frame)
	}
	if res.Ref.RefString() != "41@"+docA {
		t.Fatalf("ref = %q", res.Ref.RefString())
	}

	// The reference goes back in as the target, alone.
	if _, err := page.Click(ctx, res.Ref); err != nil {
		t.Fatal(err)
	}
	if fb.click.GetRef() != "41@"+docA || fb.click.Selector != nil || len(fb.click.In) != 0 {
		t.Fatalf("request = %+v", fb.click)
	}

	// And as a container: what it found is where the next call looks.
	if _, err := page.Click(ctx, CSS("button").In(res.Ref)); err != nil {
		t.Fatal(err)
	}
	if got := describeChain(fb.click.In); got != "ref:41@"+docA {
		t.Fatalf("chain = %q", got)
	}

	// A click at a point names no element.
	fb.clickResult = &generated.ClickResult{Success: true}
	res, err = page.Click(ctx, At(10, 20))
	if err != nil {
		t.Fatal(err)
	}
	if res.Ref != nil {
		t.Fatalf("ref for a click at a point: %v", res.Ref)
	}
	if fb.click.GetX() != 10 || fb.click.GetY() != 20 {
		t.Fatalf("request = %+v", fb.click)
	}
}

func TestWaitScopesEachConditionOnItsOwn(t *testing.T) {
	page, fb := newFakePage()
	pay := CSS("iframe[title='Secure payment']")

	_, err := page.Wait(context.Background(),
		CSS("#order-confirmed"),
		CSS(".card-error").In(pay),
		JS("location.pathname === '/declined'"),
		Captcha(),
		Captcha().In(FrameURL("*checkout*")),
		Request("*/api/*"),
	)
	if err != nil {
		t.Fatal(err)
	}
	conds := fb.wait.Conditions
	want := []string{"", "css:iframe[title='Secure payment']", "", "", "url:*checkout*", ""}
	if len(conds) != len(want) {
		t.Fatalf("%d conditions sent", len(conds))
	}
	for i, w := range want {
		if got := describeChain(conds[i].In); got != w {
			t.Errorf("condition %d: in = %q, want %q", i, got, w)
		}
	}
	if !conds[3].Captcha || !conds[4].Captcha || conds[5].Request == nil {
		t.Fatalf("conditions = %+v", conds)
	}

	for name, bad := range map[string]WaitArg{
		"a reference": Ref("7@" + docA),
		"a frame":     FrameURL("*x*"),
		"a point":     At(1, 2),
	} {
		if _, err := page.Wait(context.Background(), bad); err == nil {
			t.Errorf("%s was accepted as a wait condition", name)
		}
	}
}

func TestWaitResultNamesElementAndFrame(t *testing.T) {
	page, fb := newFakePage()
	fb.waitResult = &generated.WaitResult{
		Success: true, Index: 1, Kind: "element",
		Element: &generated.WaitElementMatch{
			Ref:   "9@" + docA,
			Frame: &generated.FrameRef{FrameId: "F2", Url: "https://pay.example/", Main: false},
		},
	}
	res, err := page.Wait(context.Background(), CSS("#a"), CSS("#b"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Element.Ref.RefString() != "9@"+docA || res.Element.Frame.URL != "https://pay.example/" {
		t.Fatalf("element = %+v", res.Element)
	}

	// A truthy value that is not an element has a frame, but nothing to act on.
	fb.waitResult = &generated.WaitResult{
		Success: true, Kind: "element",
		Element: &generated.WaitElementMatch{Frame: &generated.FrameRef{FrameId: "M", Main: true}},
	}
	res, _ = page.Wait(context.Background(), JS("true"))
	if res.Element.Ref != nil || !res.Element.Frame.Main {
		t.Fatalf("element = %+v", res.Element)
	}

	// A timeout says which container was never there.
	scope := `in[0] (selector "#nope"): no element matches.`
	fb.waitResult = &generated.WaitResult{
		Index: -1,
		Error: &generated.WaitError{Code: "timeout", Conditions: []*generated.WaitConditionStatus{
			{Index: 0, State: "not_found", ScopeError: &scope},
			{Index: 1, State: "found_hidden", Ref: "3@" + docA, FrameId: "F2"},
		}},
	}
	_, err = page.Wait(context.Background(), CSS("a").In(CSS("#nope")), CSS("b"))
	var we *WaitError
	if !errors.As(err, &we) {
		t.Fatalf("err = %v", err)
	}
	if we.Conditions[0].ScopeError != scope || we.Conditions[0].Ref != nil {
		t.Fatalf("condition 0 = %+v", we.Conditions[0])
	}
	if we.Conditions[1].Ref.RefString() != "3@"+docA {
		t.Fatalf("condition 1 = %+v", we.Conditions[1])
	}
}

func TestStaleRefSaysWhereTheFrameIsNow(t *testing.T) {
	page, fb := newFakePage()
	ctx := context.Background()
	fb.waitResult = &generated.WaitResult{
		Success: true, Kind: "element",
		Element: &generated.WaitElementMatch{
			Ref:   "9@" + docA,
			Frame: &generated.FrameRef{FrameId: "F2", Url: "https://shop.example/list"},
		},
	}
	found, _ := page.Wait(ctx, CSS(".result"))

	fb.clickResult = &generated.ClickResult{Error: &generated.ClickError{
		Code: "stale_ref", Message: "The document this element was seen in is gone.",
	}}
	fb.pages = []*generated.PageInfo{
		{PageId: "other", Url: "https://elsewhere.example/"},
		{PageId: "p1", Url: "https://shop.example/", FrameTree: &generated.FrameInfo{
			FrameId: "M", Url: "https://shop.example/",
			Children: []*generated.FrameInfo{{FrameId: "F2", Url: "https://shop.example/detail"}},
		}},
	}

	_, err := page.Click(ctx, found.Element.Ref)
	var stale *StaleRefError
	if !errors.As(err, &stale) {
		t.Fatalf("err = %v, want a StaleRefError", err)
	}
	if stale.URL != "https://shop.example/detail" {
		t.Fatalf("URL = %q, want the frame's current one", stale.URL)
	}
	if stale.Ref.RefString() != "9@"+docA {
		t.Fatalf("Ref = %v", stale.Ref)
	}
	// The call's own error is still there.
	var ce *ClickError
	if !errors.As(err, &ce) || ce.Code != "stale_ref" {
		t.Fatalf("ClickError not found in %v", err)
	}
	if !strings.Contains(err.Error(), "click failed: stale_ref") {
		t.Fatalf("message = %q", err.Error())
	}

	// The frame is gone too: the page's URL.
	fb.pages[1].FrameTree.Children = nil
	_, err = page.Click(ctx, found.Element.Ref)
	if !errors.As(err, &stale) || stale.URL != "https://shop.example/" {
		t.Fatalf("err = %v, URL = %q", err, stale.URL)
	}

	// A stale container counts the same, and Fill reports it the same way.
	fb.fillResult = &generated.FillResult{Error: &generated.FillError{
		Code: "stale_ref", ClickError: &generated.ClickError{Code: "stale_ref"},
	}}
	_, err = page.Fill(ctx, CSS("input").In(found.Element.Ref), "x")
	if !errors.As(err, &stale) || stale.Ref.RefString() != "9@"+docA {
		t.Fatalf("err = %v", err)
	}
	var fe *FillError
	if !errors.As(err, &fe) {
		t.Fatalf("FillError not found in %v", err)
	}

	// Any other failure costs no extra call and stays what it was.
	calls := fb.getPagesCalled
	fb.clickResult = &generated.ClickResult{Error: &generated.ClickError{Code: "not_found"}}
	_, err = page.Click(ctx, CSS("#gone"))
	if errors.As(err, &stale) || fb.getPagesCalled != calls {
		t.Fatalf("not_found was treated as stale: %v", err)
	}
}

func TestNotFocusedStillWins(t *testing.T) {
	page, fb := newFakePage()
	focused := "p2"
	fb.clickResult = &generated.ClickResult{Error: &generated.ClickError{
		Code: "page_not_focused", FocusedPageId: &focused,
	}}
	_, err := page.Click(context.Background(), CSS("a"))
	var nf *PageNotFocusedError
	if !errors.As(err, &nf) || nf.FocusedPage.Id() != "p2" {
		t.Fatalf("err = %v", err)
	}
}

func TestEvaluateIn(t *testing.T) {
	page, fb := newFakePage()
	ctx := context.Background()
	pay := CSS("iframe[title='Secure payment']")

	fb.evalResult = &generated.EvaluateResponse{
		Success: true, Result: `"Checkout"`,
		Frame: &generated.FrameRef{FrameId: "F2", Url: "https://pay.example/"},
	}
	res, err := page.EvaluateIn(ctx, pay, "document.title")
	if err != nil {
		t.Fatal(err)
	}
	if got := describeChain(fb.evaluate.In); got != "css:iframe[title='Secure payment']" {
		t.Fatalf("chain = %q", got)
	}
	if res.Value != "Checkout" || res.Frame.URL != "https://pay.example/" || res.Ref != nil {
		t.Fatalf("result = %+v", res)
	}

	// The container's own container comes first.
	if _, err := page.EvaluateIn(ctx, FrameURL("*inner*").In(FrameURL("*outer*")), "1"); err != nil {
		t.Fatal(err)
	}
	if got := describeChain(fb.evaluate.In); got != "url:*outer* > url:*inner*" {
		t.Fatalf("chain = %q", got)
	}

	// Plain Evaluate names no container: the main document.
	if _, err := page.Evaluate(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	if len(fb.evaluate.In) != 0 {
		t.Fatalf("Evaluate sent a chain: %q", describeChain(fb.evaluate.In))
	}

	// An element comes back as a reference.
	fb.evalResult = &generated.EvaluateResponse{Success: true, Ref: "5@" + docA, Frame: &generated.FrameRef{FrameId: "M", Main: true}}
	res, _ = page.Evaluate(ctx, "document.body")
	if res.Ref.RefString() != "5@"+docA || !res.Frame.Main {
		t.Fatalf("result = %+v", res)
	}

	for name, bad := range map[string]*Locator{"nil": nil, "any frame": AnyFrame(), "a point": At(1, 2)} {
		if _, err := page.EvaluateIn(ctx, bad, "1"); err == nil {
			t.Errorf("%s was accepted as a container", name)
		}
	}

	// A stale container is a StaleRefError around the CommandError.
	fb.evalResult = &generated.EvaluateResponse{Error: &generated.CommandError{Code: "stale_ref"}}
	_, err = page.EvaluateIn(ctx, Ref("5@"+docA), "1")
	var stale *StaleRefError
	var ce *CommandError
	if !errors.As(err, &stale) || !errors.As(err, &ce) || ce.Command != "evaluate" {
		t.Fatalf("err = %v", err)
	}
}

func TestObservationScope(t *testing.T) {
	page, fb := newFakePage()
	ctx := context.Background()
	cases := []struct {
		name         string
		scope        *Locator
		selector, in string
	}{
		{"whole page", nil, "", ""},
		{"an element", CSS("form#register"), "form#register", ""},
		{"an element in a frame", CSS("form").In(FrameURL("*checkout*")), "form", "url:*checkout*"},
		{"a frame", FrameURL("*checkout*"), "", "url:*checkout*"},
		{"an iframe element, as a frame", Frame(CSS("iframe.pay")), "", "frame(css:iframe.pay)"},
		{"the main frame", MainFrame(), "", "main"},
	}
	for _, c := range cases {
		if _, err := page.GetObservationWith(ctx, ObservationOpts{Scope: c.scope}); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := fb.observe.GetSelector(); got != c.selector {
			t.Errorf("%s: selector = %q, want %q", c.name, got, c.selector)
		}
		if got := describeChain(fb.observe.In); got != c.in {
			t.Errorf("%s: in = %q, want %q", c.name, got, c.in)
		}
	}

	if _, err := page.GetObservationWith(ctx, ObservationOpts{Scope: Ref("8@" + docA)}); err != nil {
		t.Fatal(err)
	}
	if fb.observe.GetRef() != "8@"+docA || len(fb.observe.In) != 0 {
		t.Fatalf("request = %+v", fb.observe)
	}
	if _, err := page.GetObservationWith(ctx, ObservationOpts{Scope: At(1, 2)}); err == nil {
		t.Fatal("a point was accepted as a scope")
	}
}

func TestReactionsCarryTheirContainer(t *testing.T) {
	page, fb := newFakePage()
	ctx := context.Background()

	if _, err := page.AddReaction(ctx, CSS("button#accept")); err != nil {
		t.Fatal(err)
	}
	if fb.reaction.GetMatchSelector() != "button#accept" || len(fb.reaction.In) != 0 {
		t.Fatalf("request = %+v", fb.reaction)
	}

	cmp := FrameURL("*consent.example.com/*")
	if _, err := page.AddReactionWith(ctx, CSS("#modal").In(cmp).Visible(false), ReactionOpts{On: CSS(".close")}); err != nil {
		t.Fatal(err)
	}
	if got := describeChain(fb.reaction.In); got != "url:*consent.example.com/*" {
		t.Fatalf("chain = %q", got)
	}
	if fb.reaction.GetActionSelector() != ".close" || fb.reaction.Visible == nil || *fb.reaction.Visible {
		t.Fatalf("request = %+v", fb.reaction)
	}

	for name, bad := range map[string]*Locator{"a reference": Ref("7@" + docA), "a frame": FrameURL("*x*"), "a point": At(1, 2)} {
		if _, err := page.AddReaction(ctx, bad); err == nil {
			t.Errorf("%s was accepted as a reaction match", name)
		}
	}
	if _, err := page.AddReactionWith(ctx, CSS("#m"), ReactionOpts{On: CSS(".x").In(cmp)}); err == nil {
		t.Fatal("On with a container of its own was accepted")
	}

	// What is listed is the locator that was registered.
	sel := "#modal"
	fb.reactions = []*generated.ReactionInfo{{
		ReactionId: "r1", MatchSelector: &sel, Visible: true,
		In: CSS("x").In(cmp).chain(),
	}}
	list, err := page.ListReactions(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, err = %v", list, err)
	}
	if got, want := list[0].Match.String(), `css "#modal" in frame url "*consent.example.com/*"`; got != want {
		t.Fatalf("Match = %q, want %q", got, want)
	}
}

func TestFrames(t *testing.T) {
	page, fb := newFakePage()
	ctx := context.Background()
	fb.pages = []*generated.PageInfo{{
		PageId: "p1", Url: "https://shop.example/",
		FrameTree: &generated.FrameInfo{
			FrameId: "M", Url: "https://shop.example/",
			Children: []*generated.FrameInfo{
				{FrameId: "F1", Url: "https://pay.example/", Name: "pay", IsVisible: true,
					Children: []*generated.FrameInfo{{FrameId: "F2", Url: "about:blank"}}},
				{FrameId: "F3", Url: "https://ads.example/"},
			},
		},
	}}

	frames, err := page.Frames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	frames.Walk(func(f *FrameInfo, depth int) bool {
		seen = append(seen, strings.Repeat(">", depth)+f.FrameId)
		return true
	})
	if got := strings.Join(seen, " "); got != "M >F1 >>F2 >F3" {
		t.Fatalf("walk = %q", got)
	}
	pay := frames.Children[0]
	if pay.Name != "pay" || !pay.IsVisible {
		t.Fatalf("frame = %+v", pay)
	}

	// A frame of the tree is a place to look in.
	if _, err := page.Click(ctx, CSS("button.pay").In(pay.Locator())); err != nil {
		t.Fatal(err)
	}
	if got := describeChain(fb.click.In); got != "id:F1" {
		t.Fatalf("chain = %q", got)
	}

	// Walk stops when asked to.
	n := 0
	frames.Walk(func(*FrameInfo, int) bool { n++; return n < 2 })
	if n != 2 {
		t.Fatalf("walk visited %d frames after being stopped at 2", n)
	}

	// A page that is no longer open says so the way every other call does.
	fb.pages = nil
	fb.focusedPage = "p9"
	_, err = page.Frames(ctx)
	var closed *PageClosedError
	if !errors.As(err, &closed) || closed.PageId != "p1" || closed.FocusedPage.Id() != "p9" {
		t.Fatalf("err = %v", err)
	}
}

func TestHighlightAndNodeRef(t *testing.T) {
	page, fb := newFakePage()
	ctx := context.Background()

	if err := page.Highlight(ctx, Ref("7@"+docA)); err != nil {
		t.Fatal(err)
	}
	if fb.hl.GetRef() != "7@"+docA || fb.hl.BackendNodeId != 0 {
		t.Fatalf("request = %+v", fb.hl)
	}
	if err := page.Highlight(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if fb.hl.Ref != nil {
		t.Fatalf("clearing sent a ref: %+v", fb.hl)
	}
	if err := page.Highlight(ctx, CSS("#x")); err == nil {
		t.Fatal("Highlight accepted a locator that is not a reference")
	}

	// A mirror node keeps its own way in, and can be turned into a reference.
	if err := page.HighlightNode(ctx, 12, "F1"); err != nil {
		t.Fatal(err)
	}
	if fb.hl.BackendNodeId != 12 || fb.hl.GetFrameId() != "F1" || fb.hl.Ref != nil {
		t.Fatalf("request = %+v", fb.hl)
	}
	ref, err := page.NodeRef(ctx, 12, "F1")
	if err != nil {
		t.Fatal(err)
	}
	if fb.nodeRef.BackendNodeId != 12 || fb.nodeRef.GetFrameId() != "F1" {
		t.Fatalf("request = %+v", fb.nodeRef)
	}
	if _, err := page.Click(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if fb.click.GetRef() != "12@"+docA {
		t.Fatalf("request = %+v", fb.click)
	}
}

func TestLocatorString(t *testing.T) {
	cases := map[string]*Locator{
		`css "#a"`:                             CSS("#a"),
		`css "#a" in frame url "*x*"`:          CSS("#a").In(FrameURL("*x*")),
		`js "1" in main frame`:                 JS("1").In(MainFrame()),
		`css "b" in frame(css "iframe")`:       CSS("b").In(Frame(CSS("iframe"))),
		`css "b" in css "c" in frame name "n"`: CSS("b").In(CSS("c").In(FrameName("n"))),
		`ref 7@` + docA:                        Ref("7@" + docA),
		`at (1, 2)`:                            At(1, 2),
		`css "b" in any frame in frame F1`:     CSS("b").In(AnyFrame().In(FrameByID("F1"))),
	}
	for want, l := range cases {
		if got := l.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}
