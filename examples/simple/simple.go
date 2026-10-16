package main

import (
	"context"
	"fmt"

	"github.com/browserscale/browserscale-go"
)

const (
	apiKey        = "API_KEY"
	proxyHost     = "PROXY_HOST"
	proxyPort     = 1234
	proxyUsername = "PROXY_USERNAME"
	proxyPassword = "PROXY_PASSWORD"
)

func main() {
	ctx := context.Background()

	config := browserscale.NewBrowserConfig(apiKey, 60, proxyHost, proxyPort, proxyUsername, proxyPassword)
	browser, err := browserscale.RentBrowser(ctx, config)
	if err != nil {
		fmt.Println("rent:", err)
		return
	}
	defer browser.Close()

	fmt.Println("session:", browser.SessionId())

	page, err := browser.CreatePage(ctx)
	if err != nil {
		fmt.Println("create page:", err)
		return
	}

	if _, err := page.Navigate(ctx, "https://example.com", 30000); err != nil {
		fmt.Println("navigate:", err)
		return
	}

	wr, err := page.Wait(ctx, browserscale.CSS(`a[href="https://iana.org/domains/example"]`))
	if err != nil {
		fmt.Println("wait:", err)
		return
	}
	el := wr.Element
	fmt.Printf("wait matched index=%d frame=%s node=%d visible=%v bounds=%v\n",
		wr.Index, el.FrameId, el.BackendNodeId, el.IsVisible, el.Bounds)

	if _, err := page.Click(ctx, browserscale.Node(el.BackendNodeId).InFrame(el.FrameId)); err != nil {
		fmt.Println("click:", err)
		return
	}

	if _, err := page.Wait(ctx, browserscale.CSS(`.help-article`)); err != nil {
		fmt.Println("wait:", err)
		return
	}

	fmt.Println("Test completed successfully")
}
