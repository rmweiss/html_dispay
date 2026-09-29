package main

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func connectToKiosk() (context.Context, func()) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeDebugURL)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	if err := chromedp.Run(browserCtx); err != nil {
		browserCancel()
		allocCancel()
		log.Fatalf("could not connect to Chromium: %v", err)
	}

	targets, err := chromedp.Targets(browserCtx)
	if err != nil {
		browserCancel()
		allocCancel()
		log.Fatalf("could not get Chromium targets: %v", err)
	}

	var kioskTarget target.ID
	for _, t := range targets {
		if t.Type != "page" {
			continue
		}
		log.Printf("found page: %s (%s)", t.Title, t.URL)
		if t.URL == kioskStartURL {
			kioskTarget = t.TargetID
			break
		}
	}
	if kioskTarget == "" {
		browserCancel()
		allocCancel()
		log.Fatal("could not find kiosk browser target")
	}

	log.Printf("using kiosk target %s", kioskTarget)
	chromeCtx, chromeCancel := chromedp.NewContext(browserCtx, chromedp.WithTargetID(kioskTarget))

	if err := chromedp.Run(chromeCtx, network.SetCacheDisabled(true)); err != nil {
		chromeCancel()
		browserCancel()
		allocCancel()
		log.Fatalf("could not disable browser cache: %v", err)
	}

	cleanup := func() {
		chromeCancel()
		browserCancel()
		allocCancel()
	}
	return chromeCtx, cleanup
}

func launchBrowser() (context.Context, func()) {
	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("enable-automation", false),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)

	if err := chromedp.Run(chromeCtx); err != nil {
		chromeCancel()
		allocCancel()
		log.Fatalf("could not launch Chromium: %v", err)
	}

	if err := chromedp.Run(chromeCtx, network.SetCacheDisabled(true)); err != nil {
		chromeCancel()
		allocCancel()
		log.Fatalf("could not disable browser cache: %v", err)
	}

	cleanup := func() {
		chromeCancel()
		allocCancel()
	}
	return chromeCtx, cleanup
}

func browserModeDescription(launch bool) string {
	if launch {
		return "launching a windowed local Chromium instance"
	}
	return fmt.Sprintf("attaching to Chromium at %s", chromeDebugURL)
}
