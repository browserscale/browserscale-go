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
// @throws UNKNOWN_ERROR - TURN is not configured on the server
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

// StartStream answers your WebRTC SDP offer and starts streaming the page as a
// video track, returning the SDP answer to set as your peer's remote
// description. The browser is the answerer; you are the offerer (see
// [CloudBrowser.GetStreamConfig] for the credentials to build the offer).
//
// @param offerSDP - your RTCPeerConnection's SDP offer
//
// @returns the SDP answer to apply as the remote description
//
// @throws UNKNOWN_ERROR - the offer was empty, TURN is unconfigured, or the
//
//	browser could not negotiate the stream
//
// @example
//
//	answer, err := browser.StartStream(ctx, offer.SDP)
//	if err != nil { log.Fatal(err) }
//	// peer.SetRemoteDescription({type: "answer", sdp: answer}) …
func (c *CloudBrowser) StartStream(ctx context.Context, offerSDP string) (string, error) {
	req := &generated.StartStreamRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		OfferSdp: offerSDP,
	}
	resp, err := c.client.StartStream(ctx, req)
	if resp != nil {
		return resp.AnswerSdp, err
	}
	return "", err
}

// StopStream tears down the live video stream for the session's page. It is
// safe to call even if no stream is running.
//
// @throws UNKNOWN_ERROR - the stream could not be stopped
//
// @example
//
//	if err := browser.StopStream(ctx); err != nil { log.Fatal(err) }
func (c *CloudBrowser) StopStream(ctx context.Context) error {
	_, err := c.client.StopStream(ctx, &generated.StopStreamRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	return err
}
