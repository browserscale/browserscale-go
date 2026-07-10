package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// ReadCanvasOpts customizes a [CloudBrowser.ReadCanvasWith] call.
// Zero/empty values mean "use the server default".
type ReadCanvasOpts struct {
	// InFrame overrides the locator's own frame.
	// Empty = use the locator's frame (or the main frame if none).
	// Pass a specific frameId, or [AllFrames], to search elsewhere.
	InFrame string

	// Format is the output encoding.
	// "" or "png" (default), "jpeg", "webp", or "rgba" for the raw
	// unpremultiplied RGBA pixel buffer.
	Format string

	// Quality is the encode quality 0-100 for "jpeg"/"webp" (ignored otherwise).
	// 0 = server default (90).
	Quality int32

	// SX, SY, SW, SH is an optional sub-rectangle in canvas pixels (mirrors
	// getImageData(sx, sy, sw, sh)). The full canvas is read when SW/SH <= 0.
	SX, SY, SW, SH int32
}

// ReadCanvas reads the pixels of a <canvas> element directly in the renderer,
// bypassing the origin-clean (tainted) security check and without executing any
// page JavaScript — so cross-origin/tainted canvases (common in captchas) read
// fine where a normal toDataURL / getImageData would throw a SecurityError.
//
// @param target - locator for the <canvas>; [CSS], [JS] or [Node]
//
//	([At] coordinates are not valid — a real element is required)
//
// @returns *ReadCanvasResult with the base64 image in DataBase64, the canvas
//
//	Width/Height, resolved frameId/backendNodeId and the OriginClean flag
//
// @throws INVALID_LOCATOR - target is empty, uses At(x,y), or has multiple targets
// @throws ELEMENT_NOT_FOUND - no element matched the locator
// @throws FRAME_NOT_FOUND - the requested frame does not exist
// @throws TIMEOUT - the operation exceeded the server-side timeout
// @throws PAGE_NOT_ALIVE - the page has been closed
//
// @see [CloudBrowser.ReadCanvasWith] for format, quality, or a sub-rectangle
//
// @example
//
//	res, err := browser.ReadCanvas(ctx, browserscale.CSS("#game canvas"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	img, _ := base64.StdEncoding.DecodeString(res.DataBase64)
//	os.WriteFile("canvas.png", img, 0o644)
func (c *CloudBrowser) ReadCanvas(ctx context.Context, target *Locator) (*ReadCanvasResult, error) {
	return c.readCanvasWith(ctx, target, ReadCanvasOpts{})
}

// ReadCanvasWith is the customizable variant of [CloudBrowser.ReadCanvas].
//
// @inheritDoc [CloudBrowser.ReadCanvas]
// @param opts - format, quality, sub-rectangle and frame override; see [ReadCanvasOpts]
//
// @example
//
//	// Read the left half of the canvas as JPEG at quality 80.
//	res, err := browser.ReadCanvasWith(ctx, browserscale.CSS("canvas"),
//	    browserscale.ReadCanvasOpts{Format: "jpeg", Quality: 80, SW: 150, SH: 300})
func (c *CloudBrowser) ReadCanvasWith(ctx context.Context, target *Locator, opts ReadCanvasOpts) (*ReadCanvasResult, error) {
	return c.readCanvasWith(ctx, target, opts)
}

func (c *CloudBrowser) readCanvasWith(ctx context.Context, target *Locator, o ReadCanvasOpts) (*ReadCanvasResult, error) {
	// A real <canvas> is required — At(x,y) coordinates are not valid here.
	if err := target.validateTarget("ReadCanvas", false); err != nil {
		return nil, err
	}

	req := &generated.ReadCanvasRequest{
		SessionId:     c.sessionId,
		ApiKey:        c.apiKey,
		Selector:      strPtr(target.selector),
		JsExpression:  strPtr(target.jsExpression),
		BackendNodeId: intPtr(target.backendNodeId),
		FrameId:       pickFrame(o.InFrame, target),
		Format:        strPtr(o.Format),
		Quality:       intPtr(o.Quality),
	}
	if o.SW > 0 && o.SH > 0 {
		req.Sx = intPtr(o.SX)
		req.Sy = intPtr(o.SY)
		req.Sw = intPtr(o.SW)
		req.Sh = intPtr(o.SH)
	}

	resp, err := c.client.ReadCanvas(ctx, req)
	if err != nil {
		return nil, err
	}
	return &ReadCanvasResult{
		Success:       resp.Success,
		FrameId:       resp.FrameId,
		BackendNodeId: resp.BackendNodeId,
		DataBase64:    resp.DataBase64,
		Width:         resp.Width,
		Height:        resp.Height,
		OriginClean:   resp.OriginClean,
	}, nil
}
