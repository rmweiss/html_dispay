# Local asset availability and caching: ideas

> **Status: exploratory ideas (2026-09-29).** Nothing here is implemented or required. The alternatives and policies below are candidates to test, not an agreed design.

## Motivation

The display client may use Wi-Fi. Large images and videos should ideally play from local storage when available, while newly requested content should appear immediately without waiting for a background synchronization of unrelated files. HTML and control traffic can remain independent of any asset mirroring scheme.

A stable asset URL namespace, such as `/assets/...` on the client, could let displayed pages request assets without knowing whether a file is already local. The NAS remains the source of truth. The URL convention and asset origin are not yet specified.

## Approaches discussed

| Idea | On local hit | On local miss | How local files arrive |
| --- | --- | --- | --- |
| Background mirror plus redirect | Serve local file | Redirect the browser to the NAS | An independently scheduled, low-bandwidth rsync job |
| Background mirror plus active proxy | Serve local file | Fetch from NAS and stream to browser; optionally save the complete response | Rsync and successful foreground fetches share an ordinary local asset directory |
| Conventional caching reverse proxy | Serve cached response | Proxy and cache upstream response | HTTP requests, optionally made by a background warmer |
| Explicit sync before display | Serve local file | Wait for needed file to sync | Manifest or requested-asset prefetch |

The first three allow an uncached item to display without waiting for a bulk sync. The redirect is simplest, but a foreground load does not fill the cache and the browser must reach the NAS directly. A generic proxy (for example NGINX or Apache Traffic Server) could be evaluated if HTTP caching behavior is more useful than an inspectable file mirror. Its internal cache should not be assumed to accept files copied in with rsync.

A separate warmer could request selected URLs through a caching proxy and discard the response body locally; the proxy would retain the object if its cache rules permit. A manifest/change list could prioritize new files. Alternatively, rsync could fill an ordinary directory that a small Go asset server also uses. Background work should be bandwidth-limited and yield to foreground loads. Neither mechanism should block a new display request.

## Possible narrow Go asset server

A purpose-built handler in the existing Go service could check a safe, configured local asset root, serve a complete local file, and otherwise contact one configured NAS origin. For an ordinary full GET, it could stream the upstream response to the browser and a temporary file, then atomically promote the file only after a successful complete transfer. Incomplete transfers must never become cache hits. The browser and rsync could share the final directory if their writes use a compatible atomic publication convention.

A simpler variant serves a local hit and redirects a miss. That may be sufficient if rsync reliably catches up and duplicate transfers are acceptable. A miss could also notify the background job to prioritize that path without delaying the browser.

### Range requests and video

Local files can be served with standard HTTP Range support. On a miss, forwarding `Range` to the NAS and relaying its `206 Partial Content`, `Content-Range`, and related headers should be possible, but a partial response must **not** be published as a complete cached file.

One conservative candidate policy is to proxy Range misses without caching them, leaving rsync to populate the full file. A full GET miss could stream and store concurrently. If video playback mostly issues Range requests, opportunistic foreground caching would then be uncommon. Possible later experiments include one deduplicated full background fetch per asset, or a chunk/range-aware cache; both add transfer or coordination complexity. These behaviors need testing with Chromium, the NAS server, seeking, large files, and interrupted Wi-Fi.

## Questions before implementation

- How are asset URLs mapped to NAS paths, and how are traversal, redirects, and untrusted upstream URLs excluded?
- How does the client know that a local copy is current when the NAS replaces an asset under the same path? Versioned names or a manifest/hash may make this explicit.
- How do rsync and foreground fetches publish complete files atomically, avoid competing writes, and recover from interrupted transfers?
- Should the mirror include all assets, a selected set, or only recently relevant files? What disk limit and eviction policy apply?
- What happens to a cached asset when Wi-Fi or the NAS is unavailable, and what happens on a miss?
- Does Chromium's actual video request pattern justify active caching beyond the simple redirect plus rsync design?

Start with measured browser/NAS Range behavior and the simplest scheme that meets playback needs. This note records options, not an implementation commitment.
