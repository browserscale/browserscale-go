<div align="center">

# browserscale-go

**The official Go SDK for [browserscale](https://browserscale.cloud) — real Chromium browsers in the cloud, driven over gRPC.**

Browser automation that doesn't guess. Waits, clicks and frames are handled inside the browser engine instead of being approximated from outside — rent an isolated session in under 250 ms, drive it with input that arrives like hardware, see every request it makes, and watch it live.

[![Go Reference](https://pkg.go.dev/badge/github.com/browserscale/browserscale-go.svg)](https://pkg.go.dev/github.com/browserscale/browserscale-go)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go&logoColor=white)
![License: MIT](https://img.shields.io/badge/license-MIT-blue)

[Install](#install) · [Quickstart](#quickstart) · [Core API](#core-api) · [Docs](#documentation) · [Ecosystem](#ecosystem)

</div>

---

## Features

### Acting on the page

- **Clicks that check before they press** — the element is scrolled genuinely
  into view (through nested scroll containers and up the frame chain), held
  until it stops moving, approached on a human pointer path, and the exact pixel
  is verified to belong to it — across process boundaries — before the button
  goes down. Covered? The click re-aims at the visible part or steps out of a
  hover overlay's way. Still blocked? It refuses with a `ClickError` naming the
  element in the way.
- **Input the way hardware sends it** — pointer and key events take the path a
  real mouse and keyboard take, with none of the markers of remote-controlled
  input. Typing follows the session region's keyboard layout with per-character
  timing that varies like a hand.
- **Failures you can act on** — every command answers with success or a stable
  error code (`not_found`, `occluded_after_evade`, `timeout`, …) plus typed
  detail, so code and models repair a failure instead of blindly retrying.

### Waiting & reacting

- **Waits the page reports, not a poll** — each document tells the wait the
  moment a condition holds, usually within a frame; an idle wait does no work,
  and more conditions or more frames cost a registration, not another loop. By
  default a match means visible and holding still, not merely in the DOM.
- **Timeouts that explain themselves** — a `WaitError` says per condition how
  far it got: `not_found`, `found_hidden`, `found_occluded` (with the blocker)
  or `pending_steady`.
- **Reactions** — `AddReaction` arms a one-shot handler in the browser for the
  cookie banner or popup that may or may not show up. It fires between your
  calls while the pointer is idle, across every frame and navigation, and
  retires itself — so a click blocked by a modal lands because the reaction
  cleared the modal mid-retry.

### Frames

- **One flat frame tree** — main document, same-origin iframes and cross-origin
  OOPIFs are all just a `frameId`: no per-frame sessions, no isolated worlds,
  no depth limit. A frame created mid-wait is covered the moment it exists, and
  a match returns the frame plus a node handle the next action routes on its own.

### Scripts beside the browser (BrowserVM, early access)

- **`RunScript`** runs JavaScript in its own isolate next to the page, reaching
  the document through the engine: a cross-origin `<iframe>` is plain
  `contentDocument`, values are live objects, an element goes straight into
  `browser.click`, and the page sees nothing injected. Steps cost microseconds
  instead of round trips, so loops are affordable. `StartScript` leaves a script
  running without the caller; `FollowScript` attaches to one already under way.
  [More on BrowserVM](https://browserscale.cloud/browservm).
  Access is opened per account while in early access: ask
  [support](mailto:support@browserscale.cloud) or on [Discord](https://discord.gg/SfE9C9K28D).

### Stealth on real hardware

- **Control lives below the page** — commands are carried out by the browser
  itself: nothing injected, no `Runtime.enable`, no DevTools handshake, nothing
  for page JavaScript to observe.
- **Real consumer GPUs, our own hardware** — Canvas, WebGL, audio and codec
  readbacks are genuinely rendered; there is no spoofing layer or hash database
  for deeper checks to unmask.
- **A shipped Chrome, not a build of one** — sessions carry the state and wire
  behavior of a consumer browser, consistent with the region they exit from and
  reproducible run over run.

### Network

- **Armed at the root, before anything loads** — interception sits in the
  browser's network stack, so every frame, cross-process iframe, worker and
  service worker passes through it. No attach race, nothing slips.
- **Capture that never pauses the page** — `CaptureNetwork` streams every
  finished request with the headers and cookies actually put on the wire, each
  redirect hop as its own exchange, bodies copied off to the side.
- **Catch one call and change it** — wait for a request or response, block,
  mock, rewrite headers or bodies, or answer a whole navigation yourself with
  `LoadHTML`.
- **Pay for static assets once** — `SetStaticPaths` serves heavy JS, CSS and
  images from a server-side cache reached outside the proxy, so repeat runs pay
  neither the download nor the proxy bandwidth.

### Identity & state

- **A login as one portable object** — `GetAuthSession` / `SetAuthSession`
  export a signed-in persona, device-bound sessions (DBSC) included, and bring
  it up signed in inside a fresh context.
- **Cookies and storage as data** — the whole jar, partitioned cookies
  included, and local storage per origin, read and written with no page open.
- **A machine you can come back as** — a country sets language, locale,
  timezone and keyboard together; cores, memory and renderer stay consistent in
  every frame and worker. Pin the fingerprint and the next run is the same
  computer returning. Bring your own proxy or let browserscale allocate one.

### Seeing the page

- **Agent-friendly observation** — `GetObservation` returns one line per
  element across every frame and closed shadow root, with role, live value,
  label and flags, under a token budget — prompt-sized instead of a megabyte of
  HTML, in one round trip.
- **Live DOM mirror** — `MirrorDom` keeps an incrementally updated copy of the
  page: only what changed in the part you expanded is sent, an `<iframe>` is an
  ordinary element holding its document, and `GetDomRevision` is an O(1)
  change check.
- Plus `Screenshot`, `ReadCanvas` and `InspectAtPosition`.

### Sessions at scale

- **Contexts, not machines** — a session is an isolated browser context with
  its own cookies, storage, cache, proxy and persona, ready in under 250 ms;
  thousands run side by side without sharing state.
- **Sessions you can find again** — the browser lives server-side, so a session
  outlives the process that rented it. `ListBrowsers` shows what a key holds,
  `BrowserInfo.Connect` reattaches from any machine, `StopAllBrowsers` cleans up.
- **Operated for you** — heavy sessions can't starve their neighbours, capacity
  is warm before you rent (and a full host fails fast instead of hanging), and
  sessions are rotated onto fresh processes without losing capacity.
- **Live stream and takeover** — a WebRTC stream encoded on the GPU that paints
  the page; take over with mouse, keyboard and clipboard from the dashboard or
  the CLI.
- **Captchas, no third-party solvers** — `SolveCaptcha` completes interactive
  challenges in the live session with browserscale's own solver; the provider's
  own JavaScript issues the token, nothing is synthesized or bought from an
  external API.

### Built for agents

- **MCP server** — `https://mcp.browserscale.cloud/mcp` exposes the same verbs
  as this SDK to Cursor, Claude, Codex or any MCP client; the key stays in an
  `Authorization` header, never in the model's context.
- **Idiomatic Go** — context-first methods with explicit errors, `Wait` races
  several outcomes, `JS(...)` locators target by page logic when CSS is not
  enough. `browserscale init` scaffolds a runnable module with an `AGENTS.md`
  and the SDK reference offline.

## Install

```bash
go get github.com/browserscale/browserscale-go
```

Requires **Go 1.22+**. The gRPC stubs ship precompiled — no `protoc` needed.

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/browserscale/browserscale-go"
)

func main() {
    ctx := context.Background()

    // Empty proxy fields tell browserscale to allocate a managed proxy server-side;
    // pass your own host/port/creds to bring your own.
    cfg := browserscale.NewBrowserConfig(
        "YOUR_API_KEY", // sk_…
        300,            // rent duration in seconds (5 minutes)
        "", 0, "", "",  // proxy host / port / user / pass
    )

    browser, err := browserscale.RentBrowser(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer browser.Close() // always release the session

    if _, err := browser.Navigate(ctx, "https://example.com", 0); err != nil {
        log.Fatal(err)
    }
    if _, err := browser.Wait(ctx, browserscale.CSS("h1")); err != nil {
        log.Fatal(err)
    }

    res, err := browser.Evaluate(ctx, "document.title")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("title:", res.Value)
}
```

Run it and you should see `title: Example Domain`. Get an API key from your
[dashboard](https://browserscale.cloud/dashboard/api-keys).

## Core API

Every action method is context-first and returns an explicit error. Methods come
in `Method` / `MethodWith` pairs — the plain form for the common case, the
`…With` form for an options struct.

When the page would not go along with a command, the error is a typed value with
a machine-stable `Code` — `ClickError`, `FillError`, `WaitError`, … and
`CommandError` for the rest — recovered with `errors.As`. A plain error means the
session or the connection failed, never the page.

| Method | What it does |
| --- | --- |
| `RentBrowser(ctx, cfg)` | Rent a fresh session (`NewBrowserConfig(key, secs, host, port, user, pass)`). |
| `ConnectSession(ctx, grpcURL, key, id)` | Attach to an existing session by id (from a prior rent). |
| `ListBrowsers(ctx, key)` | The sessions a key currently holds; `BrowserInfo.Connect` attaches to one, `StopAllBrowsers` releases them all. |
| `Navigate(ctx, url, timeoutMs)` | Load a URL (`0` = default timeout). |
| `Wait(ctx, locators…, opts…)` | Race one or more conditions; returns the matched index + `frameId`. |
| `Click(ctx, locator, opts…)` | Human-like click; rich `ClickError` (incl. the occluding element) on failure. |
| `FillWith(ctx, locator, text, FillOpts{})` | Per-key typing that fires real input events; `InsertText` for bulk commit. |
| `Evaluate(ctx, expr)` | Run JS in the page/frame and get a typed value back. |
| `RunScript(ctx, source)` | Run JavaScript beside the browser, where cross-origin frames are property access and every step is local; `StartScript` leaves it running, `FollowScript` watches one already going. |
| `GetObservation(ctx)` | Compact, node-handle-tagged view of the visible page across frames; `GetObservationWith` for budgets/format. |
| `CaptureNetwork(ctx, opts, onExchange)` | Stream every request the session completes, optionally with response bodies. |
| `MirrorDom(ctx, opts, onChange, onResync)` | Live, incrementally updated copy of the page's DOM across every frame. |
| `SolveCaptcha(ctx, …)` | Solve an interactive challenge in the live browser. |
| `Close()` / `StopBrowser()` | Release the rental. `CloseConn()` detaches without releasing it. |

Locators: `CSS(...)`, `JS(...)` (target by page logic when CSS can't). Plus
cookies (`GetCookies`/`SetCookies`/`ClearCookies`), storage,
auth/DBSC (`GetAuthSession`/`SetAuthSession`), network
interception, `MoveTo`/`ScrollTo`/`Drag`/`Select`/`PressKey`, and `ReadCanvas` —
see the full reference below.

## Documentation

- [Introduction](https://browserscale.cloud/docs) — what browserscale is, use cases and
  the mental model behind sessions, pages, frames and locators
- [Quickstart](https://browserscale.cloud/docs/quickstart) — from install to a
  running script in under a minute
- [Core concepts](https://browserscale.cloud/docs/concepts)
- Guides — [locators](https://browserscale.cloud/docs/guides/locators),
  [waiting](https://browserscale.cloud/docs/guides/waiting),
  [network](https://browserscale.cloud/docs/guides/network),
  [cookies](https://browserscale.cloud/docs/guides/cookies),
  [captchas](https://browserscale.cloud/docs/guides/captchas) and more
- [Go API reference](https://browserscale.cloud/docs/api-reference/go) — every
  method, type and option with runnable examples

A runnable example lives in [`examples/simple`](examples/simple).

## Ecosystem

| Project | Role |
| --- | --- |
| **browserscale-go** (you are here) | The Go SDK. |
| [**browserscale-ts**](https://github.com/browserscale/browserscale-ts) | The TypeScript SDK (Node.js + browser). |
| [**browserscale**](https://github.com/browserscale/browserscale) | The CLI: `browserscale init` scaffolds a runnable automation module, `dev` builds and streams it, `list`/`rent`/`view`/`stop` manage your cloud browsers. |
| [**browserscale-kit**](https://github.com/browserscale/browserscale-kit) | Go toolkit around the browser: config, store, queues, proxies, logging, mail. |
| [**MCP server**](https://mcp.browserscale.cloud/mcp) | The browser as tools for any MCP client, one-to-one with the SDK. |

## License

[MIT](LICENSE)
