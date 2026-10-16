<div align="center">

<img src="https://browserscale.cloud/icon-512.png" width="72" height="72" alt="browserscale">

# browserscale-go

**Browser automation that doesn't guess.**

Real Chromium in the cloud, driven from inside the engine: waits the page reports, clicks that verify before they press, every frame on one tree, and nothing for the page to detect.

[![Go Reference](https://img.shields.io/badge/go.dev-reference-0a0a0a?style=flat-square&logo=go)](https://pkg.go.dev/github.com/browserscale/browserscale-go)
[![go](https://img.shields.io/badge/go-1.22%2B-0a0a0a?style=flat-square&logo=go)](https://go.dev)
[![license](https://img.shields.io/badge/license-MIT-0a0a0a?style=flat-square)](LICENSE)

[Docs](https://browserscale.cloud/docs) · [Quickstart](https://browserscale.cloud/docs/quickstart) · [API reference](https://browserscale.cloud/docs/api-reference/go) · [Discord](https://discord.gg/SfE9C9K28D)

</div>

---

```bash
go get github.com/browserscale/browserscale-go
```

```go
func run(ctx context.Context) error {
	browser, err := browserscale.RentBrowser(ctx, browserscale.NewBrowserConfig(os.Getenv("BROWSERSCALE_API_KEY"), 300, "", 0, "", ""))
	if err != nil {
		return err
	}
	defer browser.Close()

	page, err := browser.CreatePage(ctx)
	if err != nil {
		return err
	}
	if _, err := page.Navigate(ctx, "https://example.com/login", 0); err != nil {
		return err
	}
	if _, err := page.Wait(ctx, browserscale.CSS("#email")); err != nil {
		return err
	}
	if _, err := page.Fill(ctx, browserscale.CSS("#email"), "me@example.com"); err != nil {
		return err
	}
	if _, err := page.Click(ctx, browserscale.CSS("button[type=submit]")); err != nil {
		return err // a *ClickError names what covered the button
	}

	// One wait, three ways it can end. The page reports whichever happens first.
	r, err := page.Wait(ctx,
		browserscale.CSS("#inbox"),
		browserscale.Captcha(),
		browserscale.Request("https://example.com/error*"),
	)
	if err != nil {
		return err // a *WaitError says how far each condition got
	}
	switch r.Index {
	case 1:
		_, err = page.SolveCaptcha(ctx, 0, 2)
	case 2:
		err = fmt.Errorf("rejected: %s", r.Request.Url)
	}
	return err
}
```

Get a key in your [panel](https://browserscale.cloud/panel/account). Go 1.22+, context-first, typed errors through `errors.As`. The gRPC stubs ship precompiled, so there's no `protoc` to install.

## What you won't find elsewhere

- **Waits the page reports, not a poll.** Race elements in any frame, a captcha, a request and a response in one `Wait`. A timeout tells you, per condition, how far it got: not found, hidden, or covered (and by what). [Waiting →](https://browserscale.cloud/docs/guides/waiting)
- **Clicks that check before they press.** The target is scrolled into view, held until it stops moving, and the exact pixel is verified to belong to it, even inside cross-origin frames. If something is covering it, the error names it. [Interaction →](https://browserscale.cloud/docs/guides/interaction)
- **One frame tree.** Cross-origin iframes are just another `FrameId`: no per-frame sessions, no depth limit, and a frame that appears mid-wait is covered at once. [Frames →](https://browserscale.cloud/docs/guides/frames)
- **Reactions.** Arm a one-shot handler for the cookie banner that may or may not show up. It runs in the browser at a safe point, across every frame, navigation and popup. [Reactions →](https://browserscale.cloud/docs/guides/reactions)
- **Network from the browser's own stack.** Every request, including workers and service workers, with the headers as they were sent and every redirect hop. Bodies stay in the browser until you ask for one. [Network →](https://browserscale.cloud/docs/guides/network)
- **Captchas solved in place.** browserscale's own solver, no third-party solving service. With auto-solve on, the click a challenge interrupts waits and then carries on. [Captchas →](https://browserscale.cloud/docs/guides/captchas)
- **Nothing for the page to see.** No injected scripts and no DevTools handshake. Canvas, WebGL and audio are rendered on real consumer GPUs.
- **BrowserVM** *(early access)*. Your JavaScript runs next to the page, not in it: a cross-origin iframe is plain `contentDocument`, and each step costs microseconds instead of a round trip. [BrowserVM →](https://browserscale.cloud/browservm)

Everything else, from cookies and DBSC sign-ins to the live DOM mirror and agent observations, is in the [docs](https://browserscale.cloud/docs). A runnable example lives in [`examples/simple`](examples/simple).

## Ecosystem

| | |
| --- | --- |
| [**browserscale-ts**](https://github.com/browserscale/browserscale-ts) | The same SDK for TypeScript, in Node.js and the browser |
| [**browserscale studio**](https://github.com/browserscale/browserscale) | Build bots by chatting with an agent; desktop app and `browserscale` CLI |
| [**browserscale-kit**](https://github.com/browserscale/browserscale-kit) | The Go framework Studio's bots run on: settings, jobs, proxies, a store |
| [**MCP server**](https://browserscale.cloud/docs/guides/agentic-coding) | The browser as tools for Cursor, Claude, Codex or any MCP client |

<div align="center">

[browserscale.cloud](https://browserscale.cloud) · [MIT](LICENSE)

</div>
