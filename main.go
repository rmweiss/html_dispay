package main

import (
	"context"
	"errors"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	listenAddr     = ":8080"
	chromeDebugURL = "http://127.0.0.1:9222"
	kioskStartURL  = "http://127.0.0.1:8080/"
	kioskStartupURL = "file:///etc/html-display/startup.html"
	contentURL     = "http://127.0.0.1:8080/display/content"
	contentFile    = "/var/lib/html-display/current.html"
	maxHTMLSize    = 10 * 1024 * 1024
)

type Server struct {
	chromeMu    sync.Mutex // serializes navigation and CDP reconnection
	chromeCtx   context.Context
	chromeClose func()
	launchMode  bool
	contentFile string
	mqtt        *mqttController
}

type displayURLRequest struct {
	URL string `json:"url"`
}

// reconnectKiosk must be called with chromeMu held.
func (s *Server) reconnectKiosk() error {
	log.Printf("reconnecting to Chromium kiosk target")
	ctx, closeFn, err := connectToKiosk()
	if err != nil {
		return fmt.Errorf("reconnect to Chromium failed: %w", err)
	}
	if s.chromeClose != nil {
		s.chromeClose()
	}
	s.chromeCtx = ctx
	s.chromeClose = closeFn
	log.Printf("reconnected to Chromium kiosk target")
	return nil
}

func (s *Server) navigate(url string) error {
	s.chromeMu.Lock()
	defer s.chromeMu.Unlock()

	log.Printf("navigating to %s", url)
	if !s.launchMode && s.chromeCtx.Err() != nil {
		if err := s.reconnectKiosk(); err != nil {
			return err
		}
	}
	err := chromedp.Run(s.chromeCtx, chromedp.Navigate(url))
	if err == nil || s.launchMode || (!errors.Is(err, context.Canceled) && s.chromeCtx.Err() == nil) {
		return err
	}
	log.Printf("browser context canceled during navigation: %v", err)
	if reconnectErr := s.reconnectKiosk(); reconnectErr != nil {
		return fmt.Errorf("navigation failed (%v); %w", err, reconnectErr)
	}
	log.Printf("retrying navigation to %s after reconnect", url)
	return chromedp.Run(s.chromeCtx, chromedp.Navigate(url))
}

// showLandingIfStartup never overrides a page already selected by MQTT or REST.
func (s *Server) showLandingIfStartup() error {
	s.chromeMu.Lock()
	defer s.chromeMu.Unlock()

	var currentURL string
	if err := chromedp.Run(s.chromeCtx, chromedp.Location(&currentURL)); err != nil {
		return fmt.Errorf("could not inspect kiosk URL: %w", err)
	}
	if currentURL != kioskStartupURL {
		log.Printf("leaving existing kiosk page unchanged: %s", currentURL)
		return nil
	}
	log.Print("no page selected yet; showing default landing page")
	return chromedp.Run(s.chromeCtx, chromedp.Navigate(kioskStartURL))
}

func (s *Server) setPage(url string, persist bool) error {
	if err := s.navigate(url); err != nil {
		return err
	}
	if persist && s.mqtt != nil {
		if err := s.mqtt.publishPage(url); err != nil {
			return fmt.Errorf("page displayed, but MQTT persistence failed: %w", err)
		}
	}
	return nil
}

func reachableHTTPURL(rawURL string) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q", req.URL.Scheme)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP status %s", resp.Status)
	}
	return nil
}

func (s *Server) currentPageUsable() (bool, string, error) {
	s.chromeMu.Lock()
	var currentURL string
	err := chromedp.Run(s.chromeCtx, chromedp.Location(&currentURL))
	s.chromeMu.Unlock()
	if err != nil {
		return false, "", err
	}

	if currentURL == "" || currentURL == "about:blank" || currentURL == kioskStartupURL {
		return false, currentURL, nil
	}

	if strings.HasPrefix(currentURL, "http://") || strings.HasPrefix(currentURL, "https://") {
		if err := reachableHTTPURL(currentURL); err != nil {
			return false, currentURL, nil
		}
	}

	return true, currentURL, nil
}

func (s *Server) setMQTTPage(url string) error {
	if err := reachableHTTPURL(url); err != nil {
		usable, currentURL, inspectErr := s.currentPageUsable()
		if inspectErr != nil {
			log.Printf("MQTT URL %q is not reachable: %v; could not inspect current page: %v; showing default page", url, err, inspectErr)
			return s.navigate(kioskStartURL)
		}
		if usable {
			log.Printf("MQTT URL %q is not reachable: %v; keeping current page %s", url, err, currentURL)
			return nil
		}
		log.Printf("MQTT URL %q is not reachable: %v; no usable page is active, showing default page", url, err)
		return s.navigate(kioskStartURL)
	}
	return s.setPage(url, false)
}

func (s *Server) displayURL(w http.ResponseWriter, r *http.Request) {
	var req displayURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, "invalid JSON; expected {\"url\":\"https://example.com\"}")
		return
	}
	if req.URL == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "url is required")
		return
	}
	if err := s.setPage(req.URL, true); err != nil {
		log.Printf("navigation failed: %v", err)
		s.writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("could not display URL %q: %v", req.URL, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) displayHTML(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxHTMLSize+1))
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Sprintf("could not read HTML request body: %v", err))
		return
	}
	if len(body) > maxHTMLSize {
		s.writeAPIError(w, r, http.StatusRequestEntityTooLarge, fmt.Sprintf("HTML document too large; maximum size is %d MiB", maxHTMLSize/(1024*1024)))
		return
	}
	if len(body) == 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, "HTML document is empty")
		return
	}

	dir := filepath.Dir(s.contentFile)
	tmp, err := os.CreateTemp(dir, "current-*.tmp")
	if err != nil {
		log.Printf("could not create temporary HTML file: %v", err)
		s.writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("could not store HTML in %s: %v", s.contentFile, err))
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		log.Printf("could not write HTML: %v", err)
		s.writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("could not store HTML in %s: %v", s.contentFile, err))
		return
	}
	if err := tmp.Close(); err != nil {
		log.Printf("could not close HTML file: %v", err)
		s.writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("could not store HTML in %s: %v", s.contentFile, err))
		return
	}
	if err := os.Rename(tmpName, s.contentFile); err != nil {
		log.Printf("could not replace current HTML: %v", err)
		s.writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("could not store HTML in %s: %v", s.contentFile, err))
		return
	}

	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	url := fmt.Sprintf("%s?v=%s", contentURL, hash)

	if err := s.navigate(url); err != nil {
		log.Printf("HTML stored, but navigation failed: %v", err)
		s.writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("HTML was stored successfully, but Chromium could not navigate to it: %v", err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) displayContent(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(s.contentFile); err != nil {
		if os.IsNotExist(err) {
			s.writeHTMLStatus(
				w,
				r,
				http.StatusNotFound,
				"No HTML content has been uploaded yet",
				"Upload a complete HTML document from the control page or POST it directly to /api/display/html.",
			)
			return
		}

		log.Printf("could not access stored HTML: %v", err)
		s.writeHTMLStatus(w, r, http.StatusInternalServerError, "Stored HTML is unavailable", err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, s.contentFile)
}

func main() {
	launchLocalBrowser := flag.Bool(
		"launch-browser",
		false,
		"launch a windowed local Chromium instance instead of attaching to the kiosk browser",
	)
	mqttBroker := flag.String(
		"mqtt-broker",
		"",
		"MQTT broker URL (for example tcp://192.168.1.10:1883); empty disables MQTT",
	)
	mqttPageTopic := flag.String(
		"mqtt-page-topic",
		"html-display/display/url",
		"retained MQTT topic containing the desired page URL",
	)
	showVersion := flag.Bool("version", false, "print build information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(versionInfo())
		return
	}
	log.Print(versionInfo())
	log.Printf("browser mode: %s", browserModeDescription(*launchLocalBrowser))

	var chromeCtx context.Context
	var cleanup func()
	if *launchLocalBrowser {
		chromeCtx, cleanup = launchBrowser()
	} else {
		var err error
		chromeCtx, cleanup, err = connectToKiosk()
		if err != nil {
			log.Fatalf("could not connect to kiosk: %v", err)
		}
	}

	server := &Server{
		chromeCtx: chromeCtx, chromeClose: cleanup,
		launchMode: *launchLocalBrowser, contentFile: contentFile,
	}
	defer func() {
		server.chromeMu.Lock()
		defer server.chromeMu.Unlock()
		if server.chromeClose != nil {
			server.chromeClose()
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", server.landingPage)
	mux.HandleFunc("GET /control", server.controlPage)
	mux.HandleFunc("POST /api/display/url", server.displayURL)
	mux.HandleFunc("POST /api/display/html", server.displayHTML)
	mux.HandleFunc("GET /display/content", server.displayContent)

	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("could not listen on %s: %v", listenAddr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", listenAddr)
		errCh <- httpServer.Serve(listener)
	}()

	if *launchLocalBrowser {
		if err := server.navigate(kioskStartURL); err != nil {
			log.Fatalf("could not open landing page in launched browser: %v", err)
		}
	} else {
		// Give the retained MQTT URL a chance to arrive before showing the
		// default page. Never override an already-selected destination.
		go func() {
			time.Sleep(3 * time.Second)
			if err := server.showLandingIfStartup(); err != nil {
				log.Printf("could not show default landing page: %v", err)
			}
		}()
	}

	if *mqttBroker != "" {
		mqttCtl, err := startMQTT(*mqttBroker, *mqttPageTopic, func(url string) error {
			return server.setMQTTPage(url)
		})
		if err != nil {
			log.Fatalf("could not start MQTT: %v", err)
		}
		server.mqtt = mqttCtl
		defer mqttCtl.close()
	}

	if err := <-errCh; err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
