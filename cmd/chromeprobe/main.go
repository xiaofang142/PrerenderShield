package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
	)
	allocCtx, aCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer aCancel()
	browserCtx, bCancel := chromedp.NewContext(allocCtx)
	defer bCancel()
	runCtx, rCancel := context.WithTimeout(browserCtx, 20*time.Second)
	defer rCancel()

	err := chromedp.Run(runCtx, chromedp.Navigate("about:blank"))
	fmt.Println("navigate err:", err)
}
