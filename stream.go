package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// IceServer is one entry for a WebRTC RTCPeerConnection's ICE configuration: a
// TURN (or STUN) URL plus the short-lived credentials to authenticate with it.
// Pass these to your peer before creating the SDP offer.
type IceServer struct {
	// URLs are the ICE server URLs (e.g. "turn:relay.example.com:3478?transport=udp").
	URLs []string
	// Username is the short-lived TURN REST username (empty for plain STUN).
	Username string
	// Credential is the short-lived TURN REST credential (empty for plain STUN).
	Credential string
}

// GetStreamConfig returns the ICE servers (TURN URL + short-lived credentials)
// to put in your RTCPeerConnection BEFORE creating the offer, so it can gather
// relay candidates.
//
// Live streaming is a two-step, client-offerer handshake: call GetStreamConfig,
// build your peer with the returned servers, create an offer, then pass its SDP
// to [CloudBrowser.StartStream] and apply the returned answer.
//
// @returns the ICE servers for the client RTCPeerConnection
//
// Reports a plain error when TURN is not configured on the server. That is a
// deployment condition rather than a per-call outcome, so it carries no code.
//
// @example
//
//	ice, err := browser.GetStreamConfig(ctx)
//	if err != nil { log.Fatal(err) }
//	// configure your RTCPeerConnection with ice, then create an offer …
func (c *CloudBrowser) GetStreamConfig(ctx context.Context) ([]IceServer, error) {
	resp, err := c.client.GetStreamConfig(ctx, &generated.GetStreamConfigRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	out := make([]IceServer, 0, len(resp.GetIceServers()))
	for _, s := range resp.GetIceServers() {
		out = append(out, IceServer{
			URLs:       s.GetUrls(),
			Username:   s.GetUsername(),
			Credential: s.GetCredential(),
		})
	}
	return out, nil
}

// StreamAnswer is the browser's reply to a [CloudBrowser.StartStream].
type StreamAnswer struct {
	// SDP answer to apply as your peer's remote description.
	AnswerSDP string
	// Root viewport in CSS pixels, the coordinate space the stream's input
	// data channels expect. X/Y are always 0. Map your on-screen pointer
	// positions into this space before sending them; the video may be
	// displayed at any size. It arrives with the answer rather than from a
	// separate GetPages so it cannot race the stream, and the browser pushes
	// {"type":"viewport","width":W,"height":H} on the reliable "input"
	// channel whenever it changes.
	Viewport Rect
}

// StartStream answers your WebRTC SDP offer and starts streaming the page as a
// video track. The browser is the answerer; you are the offerer (see
// [CloudBrowser.GetStreamConfig] for the credentials to build the offer).
//
// @param offerSDP - your RTCPeerConnection's SDP offer
//
// @returns the SDP answer plus the viewport to map input coordinates into
//
// @throws already_active - a stream is already running on this session; stop it
// before starting another
// @throws negotiation_failed - the browser could not agree on a connection. The
// message carries the negotiator's own diagnostic, which is usually where the
// actual cause is
//
// An empty offer or an unconfigured TURN setup is a caller mistake rather than an
// outcome, and reports as a plain error.
//
// @see [CommandError] for recovering the code with errors.As
//
// @example
//
//	stream, err := browser.StartStream(ctx, offer.SDP)
//	if err != nil { log.Fatal(err) }
//	// peer.SetRemoteDescription({type: "answer", sdp: stream.AnswerSDP}) …
func (c *CloudBrowser) StartStream(ctx context.Context, offerSDP string) (StreamAnswer, error) {
	req := &generated.StartStreamRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		OfferSdp: offerSDP,
	}
	resp, err := c.client.StartStream(ctx, req)
	if resp == nil {
		return StreamAnswer{}, err
	}
	if e := commandErrorFrom("startStream", resp.GetError()); e != nil {
		return StreamAnswer{}, e
	}
	return StreamAnswer{
		AnswerSDP: resp.GetAnswerSdp(),
		Viewport: Rect{
			X:      resp.GetViewport().GetX(),
			Y:      resp.GetViewport().GetY(),
			Width:  resp.GetViewport().GetWidth(),
			Height: resp.GetViewport().GetHeight(),
		},
	}, err
}

// StopStream tears down the live video stream for the session's page. It is
// safe to call even if no stream is running.
//
// Reports only transport failures - a dead session, a page that is gone, a broken
// connection. Stopping a stream that is not running is a no-op rather than a
// failure, so there are no error codes to branch on.
//
// @example
//
//	if err := browser.StopStream(ctx); err != nil { log.Fatal(err) }
func (c *CloudBrowser) StopStream(ctx context.Context) error {
	resp, err := c.client.StopStream(ctx, &generated.StopStreamRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		return err
	}
	return commandErrorFrom("stopStream", resp.GetError())
}
