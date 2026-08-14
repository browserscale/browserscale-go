<div align="center">

# browserscale-go

**The official Go SDK for [browserscale](https://browserscale.cloud) — real Chromium browsers in the cloud, driven over gRPC.**

Rent an isolated browser session in seconds, automate it with human-like input, intercept network traffic, solve captchas, and watch a live video stream of everything your script does.

[![Go Reference](https://pkg.go.dev/badge/github.com/browserscale/browserscale-go.svg)](https://pkg.go.dev/github.com/browserscale/browserscale-go)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go&logoColor=white)
![License: MIT](https://img.shields.io/badge/license-MIT-blue)

[Install](#install) · [Quickstart](#quickstart) · [Core API](#core-api) · [Docs](#documentation) · [Ecosystem](#ecosystem)

</div>

---

## Features

- **Real browser sessions as a service** — full Chromium in the cloud with
  pages, frames, cookies, storage and network state. No local binary.
- **Parallel isolated contexts** — each task gets its own session, fingerprint
  and lifecycle; large queues never share browser state. Sessions are browser
  contexts, not VMs or processes, so they spin up in under 250 ms and fan out to
  thousands in parallel.
- **Fingerprint & proxy handling** — pinnable server-side fingerprints,
  native Chrome control without CDP/Playwright/Puppeteer leaks, bring your own
  proxy or let browserscale allocate one.
- **Native engine-level control** — automation runs natively inside Chromium
  itself, not from outside over the DevTools protocol. Nothing is injected, no
  `Runtime.enable`, no DevTools handshake — page JS can't observe it. Waits run
  fully async with no polling loop, and built-in steady-time checks only report
  an element once it's stable in the DOM.
- **One flat frame tree** — main document, same-origin iframes and cross-origin
  OOPIFs are all just a `frameId` in one tree, no flattened sessions or
  per-frame execution-context juggling. `Wait`/`Click` act across all frames or
  a single iframe, and `Wait` returns the `frameId` that matched.
- **Human-like interaction** — mouse paths use browserscale's own movement algorithm
  instead of instant synthetic jumps.
- **WebRTC live video stream** — watch and control the rented browser live
  from the browserscale web interface; mouse and keyboard go back over data channels.
- **Captcha support, no third-party solvers** — passive anti-bot checks are
  handled automatically; interactive challenges are solved with `SolveCaptcha`
  by browserscale's own AI solver, which learns the known challenge types — puzzle,
  OCR, slide, hold and more — on its own and keeps improving as they evolve.
  No token is ever synthesized or fetched from an external API: the challenge
  is completed in the valid live browser and the provider's own JavaScript
  issues the token itself — which is why even new or unknown protections
  pass.
- **Real hardware, real GPUs** — sessions run hardware-accelerated on real
  consumer GPUs, not on VM cores with a WebGL faking layer. Canvas and WebGL
  readbacks (`toDataURL`, `getImageData`) return genuinely rendered pixels —
  no spoofing layer or fingerprint hash database for new bot protections to
  unmask.
- **Network control at the source** — interception sits in the browser's
  network stack itself, so every request from every frame (including
  cross-origin OOPIFs) passes through it; no handler races, nothing slips
  through. Wait for, block, mock or modify requests and responses without
  leaving the SDK; mark repeated assets as static with `SetStaticPaths` to
  serve them from a server-side cache and cut proxy bandwidth on repeat runs.
- **Agent-friendly observation** — `GetObservation` returns one line per visible
  element across every frame, under headers carrying the URL, title and scroll
  offset, with live form state (typed values, checkbox state, `<select>`
  options) and a node handle to act on. A model reasons over what matters
  instead of raw HTML, and doesn't need a JS round-trip to ask where it is.
- **Flow-optimized, idiomatic Go** — context-first methods with explicit
  errors, `Wait` races multiple outcomes, JS locators target elements by page
  logic when CSS is not enough.

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

| Method | What it does |
| --- | --- |
| `RentBrowser(ctx, cfg)` | Rent a fresh session (`NewBrowserConfig(key, secs, host, port, user, pass)`). |
| `ConnectSession(ctx, grpcURL, key, id)` | Attach to an existing session by id (from a prior rent). |
| `Navigate(ctx, url, timeoutMs)` | Load a URL (`0` = default timeout). |
| `Wait(ctx, locators…, opts…)` | Race one or more conditions; returns the matched index + `frameId`. |
| `Click(ctx, locator, opts…)` | Human-like click; rich `ClickError` (incl. the occluding element) on failure. |
| `FillWith(ctx, locator, text, FillOpts{})` | Per-key typing that fires real input events; `InsertText` for bulk commit. |
| `Evaluate(ctx, expr)` | Run JS in the page/frame and get a typed value back. |
| `GetObservation(ctx)` | Compact, node-handle-tagged view of the visible page across frames; `GetObservationWith` for budgets/format. |
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
| [**browserscale-cli**](https://github.com/browserscale/browserscale-cli) | `browserscale init` — scaffold a runnable automation module. |
| [**browserscale-kit**](https://github.com/browserscale/browserscale-kit) | Go toolkit around the browser: config, store, queues, proxies, logging, mail. |

## License

[MIT](LICENSE)
