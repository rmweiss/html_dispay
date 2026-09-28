# Using HTML Display

## Current behavior

The server listens on port 8080 and attaches through Chrome DevTools Protocol (CDP) to an already running Chromium instance at `127.0.0.1:9222`. At startup it looks for an existing page target whose URL is exactly `http://127.0.0.1:8080/`. If Chromium or that target is unavailable, the process exits; the service manager should start it after the kiosk and restart it as needed. It controls that same tab rather than opening a second one.

| Endpoint | Behavior |
| --- | --- |
| `GET /` | Idle and status page with the control address and stored HTML metadata; no form |
| `GET /control` | Form to send a URL or upload an HTML file |
| `POST /api/display/url` | Decode JSON with a nonempty `url`, navigate the kiosk tab, return 204 on success |
| `POST /api/display/html` | Store a nonempty raw HTML body of at most 10 MiB, navigate to the local content URL, return 204 on success |
| `GET /display/content` | Serve the latest stored HTML with `Cache-Control: no-store`, or a local 404 status page if none exists |

For example:

```sh
curl -X POST http://html-display:8080/api/display/url \
  -H 'Content-Type: application/json' \
  -d '{"url":"http://home-assistant.local:8123/"}'

curl -X POST http://html-display:8080/api/display/html \
  -H 'Content-Type: text/html; charset=utf-8' \
  --data-binary @page.html
```

The HTML file is atomically replaced at `/var/lib/html-display/current.html`; only one uploaded document is retained. Chromium navigates to `http://127.0.0.1:8080/display/content?v=<sha256>`. The hash distinguishes uploads, while the endpoint always serves the latest file. Browser caching is disabled for the controlled tab.

Uploaded HTML should be a complete document. Nonembedded CSS, JavaScript, images, audio, and video must be reachable by the browser. Use absolute URLs or an HTML `<base href="…">` for relative references. URL mode navigates directly to the requested page, which can be an interactive trusted local application.

The API is designed for a trusted local network. The current Go server has no authentication, URL allowlist, or scheme validation. It does not implement an audio-file upload endpoint. If an HTML page includes audio, playback depends on the kiosk browser and audio setup described in [deployment](deployment.md).

## Errors and persistence

Invalid JSON, a missing URL, an empty HTML body, and oversized HTML produce API errors. A failed CDP navigation produces an HTTP 500; in HTML mode the file may already have been stored. The code does not replace a failed navigation with a dedicated “Page unavailable” screen or automatically retry it.

The stored HTML file survives a server restart when `/var/lib/html-display` is persistent. The server does **not** save the last requested URL, save which mode was active, or automatically restore the prior displayed page. On kiosk startup, Chromium should open the idle page at `/`. See [plans](roadmap.md) for the desired restoration behavior.
