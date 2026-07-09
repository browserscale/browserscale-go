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
	SelectedIndex int32
	SelectedValue string
	SelectedText  string
}

// ObservationResult is the compact page snapshot returned by
// [CloudBrowser.GetObservation] — the visible, interactive elements
// rendered as prompt-friendly text and as JSON.
type ObservationResult struct {
	Text string
	Json string
}

// ScreenshotResult is a single captured image of the page, returned by
// [CloudBrowser.Screenshot]. DataBase64 holds the encoded image bytes
// (PNG by default); Width and Height are in physical pixels.
type ScreenshotResult struct {
	DataBase64 string
	Width      int32
	Height     int32
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
