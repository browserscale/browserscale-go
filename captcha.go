package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// SolveCaptcha detects and solves the first supported bot-challenge it
// finds anywhere on the page.
//
// Detection covers the common challenge types you run into in the wild.
// The challenge is completed in-page server-side (the resulting token /
// bypass cookies are wired into the page automatically), so callers can
// ignore the returned string.
//
// @param timeoutMs - how long to wait for a captcha to appear, in
//
//	milliseconds; 0 uses the server default (60s)
//
// @param retryAmount - number of retries on a failed solve before giving up
//
// @returns empty string on success — the solution is applied server-side
//
// @throws UNKNOWN_ERROR - no captcha appeared within timeoutMs, or the
//
//	detected captcha could not be solved within retryAmount attempts
//
// @example
//
//	if _, err := browser.SolveCaptcha(ctx, 0, 2); err != nil {
//	    log.Fatal(err)
//	}
func (c *CloudBrowser) SolveCaptcha(ctx context.Context, timeoutMs, retryAmount int32) (string, error) {
	resp, err := c.client.SolveCaptcha(ctx, &generated.SolveCaptchaRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		TimeoutMs:   timeoutMs,
		RetryAmount: retryAmount,
	})
	if resp != nil {
		return resp.Result, err
	}
	return "", err
}
