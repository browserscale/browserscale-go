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

func waitResultFromProto(r *generated.WaitResult) *WaitResult {
	if r == nil {
		return nil
	}
	return &WaitResult{
		Index:         r.Index,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		IsVisible:     r.IsVisible,
		Bounds:        rectFromProto(r.Bounds),
	}
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

func dragResultFromProto(r *generated.DragResult) *DragResult {
	if r == nil {
		return nil
	}
	return &DragResult{
		Success:       r.Success,
		FrameId:       r.FrameId,
		BackendNodeId: r.BackendNodeId,
		StartX:        r.StartX,
		StartY:        r.StartY,
		EndX:          r.EndX,
		EndY:          r.EndY,
	}
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
