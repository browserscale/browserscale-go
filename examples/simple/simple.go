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

	if _, err := browser.Navigate(ctx, "https://example.com", 30000); err != nil {
		fmt.Println("navigate:", err)
		return
	}

	wr, err := browser.Wait(ctx, browserscale.CSS(`a[href="https://iana.org/domains/example"]`))
	if err != nil {
		fmt.Println("wait:", err)
		return
	}
	fmt.Printf("wait matched index=%d frame=%s node=%d visible=%v bounds=%v\n",
		wr.Index, wr.FrameId, wr.BackendNodeId, wr.IsVisible, wr.Bounds)

	if _, err := browser.Click(ctx, browserscale.Node(wr.BackendNodeId).InFrame(wr.FrameId)); err != nil {
		fmt.Println("click:", err)
		return
	}

	if _, err := browser.Wait(ctx, browserscale.CSS(`.help-article`)); err != nil {
		fmt.Println("wait:", err)
		return
	}

	fmt.Println("Test completed successfully")
}
