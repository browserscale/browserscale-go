package browserscale

import (
	"github.com/browserscale/browserscale-go/generated"
)

// ── proto -> SDK ──

func rectFromProto(r *generated.Rect) Rect {
	if r == nil {
		return Rect{}
	}
	return Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}

func frameInfoFromProto(f *generated.FrameInfo) *FrameInfo {
	if f == nil {
		return nil
	}
	out := &FrameInfo{
		FrameId:      f.FrameId,
		Url:          f.Url,
		IsOOPIF:      f.IsOopif,
		HasJSContext: f.HasJsContext,
		IsLoading:    f.IsLoading,
		IsVisible:    f.IsVisible,
		AbsoluteRect: rectFromProto(f.AbsoluteRect),
		RelativeRect: rectFromProto(f.RelativeRect),
	}
	for _, c := range f.Children {
		out.Children = append(out.Children, frameInfoFromProto(c))
	}
	return out
}

func pageInfoFromProto(p *generated.PageInfo) *PageInfo {
	if p == nil {
		return nil
	}
	out := &PageInfo{
		PageId:           p.PageId,
		BrowserContextId: p.BrowserContextId,
		Url:              p.Url,
		Title:            p.Title,
		Viewport:         rectFromProto(p.Viewport),
	}
	if f := frameInfoFromProto(p.FrameTree); f != nil {
		out.FrameTree = *f
	}
	return out
}

func headersFromProto(hs []*generated.Header) []Header {
	if len(hs) == 0 {
		return nil
	}
	out := make([]Header, len(hs))
	for i, h := range hs {
		out[i] = Header{Name: h.Name, Value: h.Value}
	}
	return out
}

func interceptedRequestFromProto(r *generated.InterceptedRequest) *InterceptedRequest {
	if r == nil {
		return nil
	}
	return &InterceptedRequest{
		Method:       r.Method,
		Url:          r.Url,
		Headers:      headersFromProto(r.Headers),
		Body:         r.Body,
		ResourceType: r.ResourceType,
	}
}

func interceptedResponseFromProto(r *generated.InterceptedResponse) *InterceptedResponse {
	if r == nil {
		return nil
	}
	return &InterceptedResponse{
		Url:        r.Url,
		StatusCode: r.StatusCode,
		Headers:    headersFromProto(r.Headers),
		Body:       r.Body,
	}
}

func cookiesFromProto(cs []*generated.CookieParam) []CookieParam {
	if len(cs) == 0 {
		return nil
	}
	out := make([]CookieParam, len(cs))
	for i, c := range cs {
		out[i] = CookieParam{
			Name:         c.Name,
			Value:        c.Value,
			URL:          c.Url,
			Domain:       c.Domain,
			Path:         c.Path,
			Secure:       c.Secure,
			HTTPOnly:     c.HttpOnly,
			SameSite:     c.SameSite,
			Expires:      c.Expires,
			Priority:     c.Priority,
			SourceScheme: c.SourceScheme,
			SourcePort:   intPtrToInt(c.SourcePort),
			PartitionKey: cookiePartitionKeyFromProto(c.PartitionKey),
		}
	}
	return out
}

func cookiePartitionKeyFromProto(k *generated.CookiePartitionKey) *CookiePartitionKey {
	if k == nil {
		return nil
	}
	return &CookiePartitionKey{
		TopLevelSite:         k.TopLevelSite,
		HasCrossSiteAncestor: k.HasCrossSiteAncestor,
	}
}

func storageFromProto(es []*generated.StorageOriginEntry) []StorageOriginEntry {
	if len(es) == 0 {
		return nil
	}
	out := make([]StorageOriginEntry, len(es))
	for i, e := range es {
		items := make([]StorageItem, len(e.Items))
		for j, it := range e.Items {
			items[j] = StorageItem{Key: it.Key, Value: it.Value}
		}
		out[i] = StorageOriginEntry{Origin: e.Origin, Items: items}
	}
	return out
}

// waitResultFromProto splits a WaitResult into the res, err shape used by the
// SDK: the match payload always, plus a typed *WaitError when no condition
// matched before the deadline (index=-1 with an error detail). The returned
// error is nil on a match.
func waitResultFromProto(r *generated.WaitResult) (*WaitResult, *WaitError) {
	if r == nil {
		return nil, nil
	}
	res := &WaitResult{
		Index:         r.Index,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		IsVisible:     r.IsVisible,
		Bounds:        rectFromProto(r.Bounds),
	}
	if r.Error == nil {
		return res, nil
	}
	return res, waitErrorFromProto(r.Error)
}

func waitErrorFromProto(e *generated.WaitError) *WaitError {
	if e == nil {
		return nil
	}
	out := &WaitError{
		Code:    e.Code,
		Message: e.Message,
	}
	for _, c := range e.Conditions {
		out.Conditions = append(out.Conditions, waitConditionStatusFromProto(c))
	}
	return out
}

func waitConditionStatusFromProto(c *generated.WaitConditionStatus) WaitConditionStatus {
	if c == nil {
		return WaitConditionStatus{}
	}
	out := WaitConditionStatus{
		Index:         c.Index,
		State:         c.State,
		BackendNodeId: c.BackendNodeId,
		FrameId:       c.FrameId,
		IsVisible:     c.IsVisible,
		Occluder:      occluderInfoFromProto(c.Occluder),
	}
	if c.Bounds != nil {
		b := rectFromProto(c.Bounds)
		out.Bounds = &b
	}
	return out
}

func elementResultFromProto(r *generated.ElementResult) *ElementResult {
	if r == nil {
		return nil
	}
	return &ElementResult{
		Success:       r.Success,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		IsVisible:     r.IsVisible,
		Bounds:        rectFromProto(r.Bounds),
		RootX:         r.RootX,
		RootY:         r.RootY,
	}
}

// clickResultFromProto splits a ClickResult into the res, err shape used by the
// SDK: the ElementResult payload always, plus a typed *ClickError when the click
// did not land (success=false with an error detail). The returned error is nil
// on success.
func clickResultFromProto(r *generated.ClickResult) (*ElementResult, *ClickError) {
	if r == nil {
		return nil, nil
	}
	res := &ElementResult{
		Success:       r.Success,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		IsVisible:     r.IsVisible,
		Bounds:        rectFromProto(r.Bounds),
		RootX:         r.RootX,
		RootY:         r.RootY,
	}
	if r.Success || r.Error == nil {
		return res, nil
	}
	return res, clickErrorFromProto(r.Error)
}

func clickErrorFromProto(e *generated.ClickError) *ClickError {
	if e == nil {
		return nil
	}
	return &ClickError{
		Code:           e.Code,
		Message:        e.Message,
		Occluder:       occluderInfoFromProto(e.Occluder),
		EvadeAttempted: e.GetEvadeAttempted(),
	}
}

// fillResultFromProto splits a FillResult into the res, err shape used by the
// SDK: the ElementResult payload always (is_visible/bounds are not part of
// fill, so they stay zero), plus a typed *FillError when the field could not be
// focused/typed. The returned error is nil on success.
func fillResultFromProto(r *generated.FillResult) (*ElementResult, *FillError) {
	if r == nil {
		return nil, nil
	}
	res := &ElementResult{
		Success:       r.Success,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		RootX:         r.RootX,
		RootY:         r.RootY,
	}
	if r.Success || r.Error == nil {
		return res, nil
	}
	return res, fillErrorFromProto(r.Error)
}

func fillErrorFromProto(e *generated.FillError) *FillError {
	if e == nil {
		return nil
	}
	return &FillError{
		Code:       e.Code,
		Message:    e.Message,
		ClickError: clickErrorFromProto(e.ClickError),
	}
}

func occluderInfoFromProto(o *generated.OccluderInfo) *OccluderInfo {
	if o == nil {
		return nil
	}
	return &OccluderInfo{
		BackendNodeId:          o.BackendNodeId,
		FrameId:                o.FrameId,
		TagName:                o.TagName,
		Id:                     o.GetId(),
		ClassName:              o.GetClassName(),
		Text:                   o.GetText(),
		Bounds:                 rectFromProto(o.Bounds),
		PointerEvents:          o.GetPointerEvents(),
		Visibility:             o.GetVisibility(),
		Opacity:                o.GetOpacity(),
		ZIndex:                 o.GetZIndex(),
		HittableWhileInvisible: o.GetHittableWhileInvisible(),
	}
}

// dragResultFromProto splits a DragResult into the res, err shape: the drag
// payload always, plus a typed *DragError when the source pickup failed. The
// returned error is nil on success.
func dragResultFromProto(r *generated.DragResult) (*DragResult, *DragError) {
	if r == nil {
		return nil, nil
	}
	res := &DragResult{
		Success:       r.Success,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		StartX:        r.StartX,
		StartY:        r.StartY,
		EndX:          r.EndX,
		EndY:          r.EndY,
	}
	if r.Success || r.Error == nil {
		return res, nil
	}
	return res, &DragError{
		Code:       r.Error.Code,
		Message:    r.Error.Message,
		ClickError: clickErrorFromProto(r.Error.ClickError),
	}
}

// scrollResultFromProto splits a ScrollResult into the res, err shape: the
// ElementResult payload always (scrollTo returns no root_x/root_y), plus a
// typed *ScrollError when the target could not be located/scrolled.
func scrollResultFromProto(r *generated.ScrollResult) (*ElementResult, *ScrollError) {
	if r == nil {
		return nil, nil
	}
	res := &ElementResult{
		Success:       r.Success,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		IsVisible:     r.IsVisible,
		Bounds:        rectFromProto(r.Bounds),
	}
	if r.Success || r.Error == nil {
		return res, nil
	}
	return res, &ScrollError{Code: r.Error.Code, Message: r.Error.Message}
}

// moveResultFromProto splits a MoveResult into the res, err shape: the
// ElementResult payload always, plus a typed *MoveError when the target could
// not be located.
func moveResultFromProto(r *generated.MoveResult) (*ElementResult, *MoveError) {
	if r == nil {
		return nil, nil
	}
	res := &ElementResult{
		Success:       r.Success,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		IsVisible:     r.IsVisible,
		Bounds:        rectFromProto(r.Bounds),
		RootX:         r.RootX,
		RootY:         r.RootY,
	}
	if r.Success || r.Error == nil {
		return res, nil
	}
	return res, &MoveError{Code: r.Error.Code, Message: r.Error.Message}
}

// selectOptionResultFromProto splits a SelectOptionResult into the res, err
// shape: the selection payload always, plus a typed *SelectOptionError when no
// option was selected.
func selectOptionResultFromProto(r *generated.SelectOptionResult) (*SelectOptionResult, *SelectOptionError) {
	if r == nil {
		return nil, nil
	}
	res := &SelectOptionResult{
		Success:       r.Success,
		SelectedIndex: r.SelectedIndex,
		SelectedValue: r.SelectedValue,
		SelectedText:  r.SelectedText,
	}
	if r.Error == nil {
		return res, nil
	}
	return res, &SelectOptionError{Code: r.Error.Code, Message: r.Error.Message}
}

// ── SDK -> proto ──

func cookiesToProto(cs []CookieParam) []*generated.CookieParam {
	if len(cs) == 0 {
		return nil
	}
	out := make([]*generated.CookieParam, len(cs))
	for i, c := range cs {
		out[i] = &generated.CookieParam{
			Name:         c.Name,
			Value:        c.Value,
			Url:          c.URL,
			Domain:       c.Domain,
			Path:         c.Path,
			Secure:       c.Secure,
			HttpOnly:     c.HTTPOnly,
			SameSite:     c.SameSite,
			Expires:      c.Expires,
			Priority:     c.Priority,
			SourceScheme: c.SourceScheme,
			SourcePort:   intPtrFromInt(c.SourcePort),
			PartitionKey: cookiePartitionKeyToProto(c.PartitionKey),
		}
	}
	return out
}

func cookiePartitionKeyToProto(k *CookiePartitionKey) *generated.CookiePartitionKey {
	if k == nil {
		return nil
	}
	return &generated.CookiePartitionKey{
		TopLevelSite:         k.TopLevelSite,
		HasCrossSiteAncestor: k.HasCrossSiteAncestor,
	}
}

func storageToProto(es []StorageOriginEntry) []*generated.StorageOriginEntry {
	if len(es) == 0 {
		return nil
	}
	out := make([]*generated.StorageOriginEntry, len(es))
	for i, e := range es {
		items := make([]*generated.StorageItem, len(e.Items))
		for j, it := range e.Items {
			items[j] = &generated.StorageItem{Key: it.Key, Value: it.Value}
		}
		out[i] = &generated.StorageOriginEntry{Origin: e.Origin, Items: items}
	}
	return out
}

func headersToProto(hs []Header) []*generated.Header {
	if len(hs) == 0 {
		return nil
	}
	out := make([]*generated.Header, len(hs))
	for i, h := range hs {
		out[i] = &generated.Header{Name: h.Name, Value: h.Value}
	}
	return out
}

func headerModsToProto(mods []HeaderModification) []*generated.HeaderModification {
	out := make([]*generated.HeaderModification, len(mods))
	for i, m := range mods {
		pm := &generated.HeaderModification{Name: m.Name, Action: string(m.Action)}
		if m.Value != "" {
			v := m.Value
			pm.Value = &v
		}
		if m.Before != "" {
			b := m.Before
			pm.Before = &b
		}
		if m.After != "" {
			a := m.After
			pm.After = &a
		}
		out[i] = pm
	}
	return out
}

// stringPtr / boolPtr / int32Ptr / float64Ptr — small helpers for proto optionals.

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(v int32) *int32 {
	if v == 0 {
		return nil
	}
	return &v
}

func intPtrFromInt(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func intPtrToInt(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

func floatPtrIfNonZero(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}
