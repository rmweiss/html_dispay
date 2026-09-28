package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

const (
	listenAddr     = ":8080"
	chromeDebugURL = "http://127.0.0.1:9222"
	kioskStartURL  = "http://127.0.0.1:8080/"
	contentURL     = "http://127.0.0.1:8080/display/content"
	contentFile    = "/var/lib/html-display/current.html"
	maxHTMLSize    = 10 * 1024 * 1024
)

type Server struct {
	chromeCtx   context.Context
	contentFile string
}

type displayURLRequest struct {
	URL string `json:"url"`
}

func (s *Server) navigate(url string) error {
	log.Printf("navigating to %s", url)
	return chromedp.Run(s.chromeCtx, chromedp.Navigate(url))
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
	if err := s.navigate(req.URL); err != nil {
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

func connectToKiosk() (context.Context, context.CancelFunc, context.CancelFunc) {
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

	cleanupBrowser := func() {
		browserCancel()
		allocCancel()
	}
	return chromeCtx, chromeCancel, cleanupBrowser
}

func main() {
	chromeCtx, chromeCancel, cleanupBrowser := connectToKiosk()
	defer chromeCancel()
	defer cleanupBrowser()

	server := &Server{chromeCtx: chromeCtx, contentFile: contentFile}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", server.landingPage)
	mux.HandleFunc("GET /control", server.controlPage)
	mux.HandleFunc("POST /api/display/url", server.displayURL)
	mux.HandleFunc("POST /api/display/html", server.displayHTML)
	mux.HandleFunc("GET /display/content", server.displayContent)

	log.Printf("listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}
