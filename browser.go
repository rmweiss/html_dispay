package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// devtoolsPage describes the /json/list response, whose "id" field differs
// from the CDP TargetInfo "targetId" field.
type devtoolsPage struct {
	ID    target.ID `json:"id"`
	Type  string    `json:"type"`
	Title string    `json:"title"`
	URL   string    `json:"url"`
}

func connectToKiosk() (context.Context, func(), error) {
	// Discover the existing target over HTTP before creating any chromedp
	// context. Running an unbound context creates an unwanted blank tab.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(chromeDebugURL + "/json/list")
	if err != nil {
		return nil, nil, fmt.Errorf("could not list Chromium targets: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("could not list Chromium targets: HTTP %s", resp.Status)
	}

	var targets []devtoolsPage
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, nil, fmt.Errorf("could not decode Chromium targets: %w", err)
	}

	var kioskTarget target.ID
	var navigatedTarget target.ID
	var navigatedCount int
	for _, t := range targets {
		if t.Type != "page" {
			continue
		}
		log.Printf("found page: %s (%s)", t.Title, t.URL)
		if t.URL == kioskStartURL {
			kioskTarget = t.ID
			break
		}
		if t.URL != "" && t.URL != "about:blank" {
			navigatedTarget = t.ID
			navigatedCount++
		}
	}
	if kioskTarget == "" && navigatedCount == 1 {
		kioskTarget = navigatedTarget
		log.Printf("recovered navigated kiosk target %s", kioskTarget)
	}
	if kioskTarget == "" {
		return nil, nil, fmt.Errorf("could not find kiosk browser target (nonblank candidates: %d)", navigatedCount)
	}

	log.Printf("using kiosk target %s", kioskTarget)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeDebugURL)
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx, chromedp.WithTargetID(kioskTarget))
	if err := chromedp.Run(chromeCtx, network.SetCacheDisabled(true)); err != nil {
		chromeCancel()
		allocCancel()
		return nil, nil, fmt.Errorf("could not attach to kiosk target: %w", err)
	}

	cleanup := func() {
		chromeCancel()
		allocCancel()
	}
	return chromeCtx, cleanup, nil
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
