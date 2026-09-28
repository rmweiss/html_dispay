# HTML Display

Minimal Go control server for a dedicated Chromium kiosk display.

See the [documentation](docs/README.md) for current behavior, deployment notes, and planned work.

## Pages

- `GET /` shows an informational landing/status page
- `GET /control` shows a separate control form for sending a URL or HTML file
- the control form uses only vanilla JavaScript and the same REST API exposed to other clients

The landing page intentionally does **not** contain the control form.

## API endpoints

- `POST /api/display/url` with JSON containing a `url`
- `POST /api/display/html` with the complete HTML document as the raw request body
- `GET /display/content` serves the current HTML document

API errors are returned as plain text with the HTTP status plus links/addresses for the control page and both display endpoints.

Uploaded HTML is stored at:

```text
/var/lib/html-display/current.html
```

The file is replaced atomically through a temporary file and rename. Only the current document is retained.

The server attaches to an existing Chromium instance through CDP on `127.0.0.1:9222`.

## Examples

Display a URL:

```bash
curl -X POST http://html-display:8080/api/display/url \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

Display an HTML file:

```bash
curl -X POST http://html-display:8080/api/display/html \
  -H 'Content-Type: text/html; charset=utf-8' \
  --data-binary @page.html
```

## Build

```bash
CGO_ENABLED=0 go build -o html-display-server .
```

The binary is intended to be installed at:

```text
/opt/html-display/html-display-server
```

On NixOS/systemd, the persistent state directory can be managed with:

```nix
StateDirectory = "html-display";
```

which creates and manages `/var/lib/html-display` for the service.
