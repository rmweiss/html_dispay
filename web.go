package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

type pageInfo struct {
	Hostname          string
	ControlURL        string
	DisplayContentURL string
	URLAPI            string
	HTMLAPI           string
	StoredHTML        bool
	StoredHTMLSize    int64
	StoredHTMLModTime string
}

func (s *Server) landingPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := landingTemplate.Execute(w, s.pageInfo(r)); err != nil {
		log.Printf("could not render landing page: %v", err)
	}
}

func (s *Server) controlPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := controlTemplate.Execute(w, s.pageInfo(r)); err != nil {
		log.Printf("could not render control page: %v", err)
	}
}

func (s *Server) pageInfo(r *http.Request) pageInfo {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "html-display"
	}

	baseURL := "http://" + hostname + ":8080"
	if r.Host != "" && !strings.HasPrefix(r.Host, "127.0.0.1") && !strings.HasPrefix(r.Host, "localhost") {
		baseURL = "http://" + r.Host
	}

	info := pageInfo{
		Hostname:          hostname,
		ControlURL:        baseURL + "/control",
		DisplayContentURL: baseURL + "/display/content",
		URLAPI:            baseURL + "/api/display/url",
		HTMLAPI:           baseURL + "/api/display/html",
	}

	if stat, err := os.Stat(s.contentFile); err == nil {
		info.StoredHTML = true
		info.StoredHTMLSize = stat.Size()
		info.StoredHTMLModTime = stat.ModTime().Format("2006-01-02 15:04:05")
	}

	return info
}

func (s *Server) writeAPIError(w http.ResponseWriter, r *http.Request, status int, message string) {
	info := s.pageInfo(r)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(
		w,
		"%s\n\nHTTP %d %s\nControl page: %s\nURL endpoint: %s\nHTML endpoint: %s\n",
		message,
		status,
		http.StatusText(status),
		info.ControlURL,
		info.URLAPI,
		info.HTMLAPI,
	)
}

func (s *Server) writeHTMLStatus(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	data := struct {
		Status     int
		Title      string
		Detail     string
		ControlURL string
	}{
		Status:     status,
		Title:      title,
		Detail:     detail,
		ControlURL: s.pageInfo(r).ControlURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := statusTemplate.Execute(w, data); err != nil {
		log.Printf("could not render status page: %v", err)
	}
}

var landingTemplate = template.Must(template.New("landing").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>HTML Display</title>
<style>
:root { color-scheme: dark; font-family: system-ui, sans-serif; }
body { margin: 0; background: #111; color: #eee; }
main { max-width: 900px; margin: 8vh auto; padding: 0 32px 48px; }
h1 { font-size: clamp(42px, 7vw, 80px); margin: 0 0 12px; }
h2 { margin: 0 0 10px; }
p, li { font-size: 20px; line-height: 1.5; }
code { background: #222; padding: .15em .35em; border-radius: 4px; }
a { color: #9bc8ff; }
.card { border: 1px solid #333; border-radius: 12px; padding: 20px 24px; margin-top: 24px; background: #181818; }
.muted { color: #aaa; }
</style>
</head>
<body>
<main>
<h1>HTML Display</h1>
<p>The display service is running on <strong>{{.Hostname}}</strong>.</p>

<div class="card">
<h2>Control this display</h2>
<p>Open <a href="{{.ControlURL}}">{{.ControlURL}}</a> from another device on the local network.</p>
<p class="muted">The control form lives on a separate page so this landing page stays useful as an idle and status screen.</p>
</div>

<div class="card">
<h2>HTTP API</h2>
<ul>
<li><code>POST {{.URLAPI}}</code> with JSON <code>{"url":"https://example.com"}</code></li>
<li><code>POST {{.HTMLAPI}}</code> with a complete HTML document as the raw request body</li>
<li><code>GET {{.DisplayContentURL}}</code> serves the currently stored HTML document</li>
</ul>
</div>

<div class="card">
<h2>Stored HTML</h2>
{{if .StoredHTML}}
<p>Current HTML is available: {{.StoredHTMLSize}} bytes, last updated {{.StoredHTMLModTime}}.</p>
{{else}}
<p>No HTML document is currently stored.</p>
{{end}}
</div>
</main>
</body>
</html>`))

var controlTemplate = template.Must(template.New("control").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Control · HTML Display</title>
<style>
:root { color-scheme: dark; font-family: system-ui, sans-serif; }
body { margin: 0; background: #111; color: #eee; }
main { max-width: 760px; margin: 48px auto; padding: 0 24px 48px; }
h1 { font-size: 42px; margin-bottom: 8px; }
a { color: #9bc8ff; }
section { border: 1px solid #333; border-radius: 12px; background: #181818; padding: 22px; margin-top: 24px; }
label { display: block; font-weight: 650; margin-bottom: 8px; }
input[type=url], input[type=file] { width: 100%; box-sizing: border-box; font: inherit; }
input[type=url] { padding: 10px 12px; border: 1px solid #555; border-radius: 7px; background: #111; color: #eee; }
button { margin-top: 14px; padding: 10px 18px; font: inherit; font-weight: 650; cursor: pointer; }
.status { white-space: pre-wrap; margin-top: 16px; min-height: 1.4em; }
.status.error { color: #ff9e9e; }
.status.ok { color: #a8e6a1; }
.muted { color: #aaa; }
</style>
</head>
<body>
<main>
<p><a href="/">← Status</a></p>
<h1>Control HTML Display</h1>
<p class="muted">Send a URL or upload a complete HTML document. A successful request immediately replaces what is shown on the kiosk display.</p>

<section>
<form id="url-form">
<label for="url">Display a URL</label>
<input id="url" name="url" type="url" placeholder="https://example.com" required>
<button type="submit">Display URL</button>
<div id="url-status" class="status" aria-live="polite"></div>
</form>
</section>

<section>
<form id="html-form">
<label for="html-file">Display an HTML file</label>
<input id="html-file" name="file" type="file" accept="text/html,.html,.htm" required>
<button type="submit">Display HTML</button>
<div id="html-status" class="status" aria-live="polite"></div>
</form>
</section>

<script>
function setStatus(element, message, ok) {
  element.textContent = message;
  element.className = "status " + (ok ? "ok" : "error");
}

document.getElementById("url-form").addEventListener("submit", async event => {
  event.preventDefault();

  const status = document.getElementById("url-status");
  const url = document.getElementById("url").value.trim();
  setStatus(status, "Sending…", true);

  try {
    const response = await fetch("/api/display/url", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url })
    });

    if (!response.ok) {
      throw new Error((await response.text()).trim() || response.statusText);
    }

    setStatus(status, "URL sent to the display.", true);
  } catch (error) {
    setStatus(status, error.message, false);
  }
});

document.getElementById("html-form").addEventListener("submit", async event => {
  event.preventDefault();

  const status = document.getElementById("html-status");
  const file = document.getElementById("html-file").files[0];
  if (!file) return;
  setStatus(status, "Uploading…", true);

  try {
    const response = await fetch("/api/display/html", {
      method: "POST",
      headers: { "Content-Type": "text/html; charset=utf-8" },
      body: file
    });

    if (!response.ok) {
      throw new Error((await response.text()).trim() || response.statusText);
    }

    setStatus(status, "HTML sent to the display.", true);
  } catch (error) {
    setStatus(status, error.message, false);
  }
});
</script>
</main>
</body>
</html>`))

var statusTemplate = template.Must(template.New("status").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · HTML Display</title>
<style>
:root { color-scheme: dark; font-family: system-ui, sans-serif; }
body { margin: 0; background: #111; color: #eee; }
main { max-width: 820px; margin: 10vh auto; padding: 0 32px; }
h1 { font-size: clamp(38px, 6vw, 68px); }
p { font-size: 20px; line-height: 1.5; }
a { color: #9bc8ff; }
.status { color: #aaa; }
</style>
</head>
<body>
<main>
<p class="status">HTTP {{.Status}}</p>
<h1>{{.Title}}</h1>
<p>{{.Detail}}</p>
<p>Control this display at <a href="{{.ControlURL}}">{{.ControlURL}}</a>.</p>
</main>
</body>
</html>`))
