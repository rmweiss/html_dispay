# HTML Display

Minimal Go control server for a dedicated Chromium kiosk display.

## Endpoints

- `POST /api/display/url` with JSON containing a `url`
- `POST /api/display/html` with the complete HTML document as the request body
- `GET /display/content` serves the current HTML document

Uploaded HTML is stored at `/var/lib/html-display/current.html`.

The server attaches to an existing Chromium instance through CDP on `127.0.0.1:9222`.

## Build

```bash
CGO_ENABLED=0 go build -o html-display-server .
```

The binary is intended to be installed at `/opt/html-display/html-display-server`.

On NixOS/systemd, the persistent state directory can be managed with:

```nix
StateDirectory = "html-display";
```
