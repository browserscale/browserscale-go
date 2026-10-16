// Package browserscale is the official Go SDK for browserscale —
// real Chromium browsers in the cloud, driven over gRPC.
//
// Rent an isolated browser session, navigate, interact with the page using
// human-like input, intercept network traffic, manage cookies, and stream
// live video — all from idiomatic, context-first Go.
//
// A minimal end-to-end program:
//
//	ctx := context.Background()
//
//	cfg := browserscale.NewBrowserConfig("YOUR_API_KEY", 300, "", 0, "", "")
//	browser, err := browserscale.RentBrowser(ctx, cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer browser.Close()
//
//	page, err := browser.CreatePage(ctx) // a browser starts without a page
//	if err != nil {
//		log.Fatal(err)
//	}
//	if _, err := page.Navigate(ctx, "https://example.com", 0); err != nil {
//		log.Fatal(err)
//	}
//	if _, err := page.Wait(ctx, browserscale.CSS("h1")); err != nil {
//		log.Fatal(err)
//	}
//
//	res, err := page.Evaluate(ctx, "document.title")
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println("title:", res.Value)
//
// Full documentation, guides and the complete API reference live at
// https://browserscale.cloud/docs.
package browserscale
