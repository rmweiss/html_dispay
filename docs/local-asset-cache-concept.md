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
| Local HTTP server over NFS with FS-Cache | Read cached data through the NFS client | Read needed data from the NAS through NFS | Filesystem reads, optionally from a background warmer |
| Explicit sync before display | Serve local file | Wait for needed file to sync | Manifest or requested-asset prefetch |

The first three allow an uncached item to display without waiting for a bulk sync. The redirect is simplest, but a foreground load does not fill the cache and the browser must reach the NAS directly. A generic proxy (for example NGINX or Apache Traffic Server) could be evaluated if HTTP caching behavior is more useful than an inspectable file mirror. Its internal cache should not be assumed to accept files copied in with rsync.

A separate warmer could request selected URLs through a caching proxy and discard the response body locally; the proxy would retain the object if its cache rules permit. A manifest/change list could prioritize new files. Alternatively, rsync could fill an ordinary directory that a small Go asset server also uses. Background work should be bandwidth-limited and yield to foreground loads. Neither mechanism should block a new display request.

## Possible narrow Go asset server

A purpose-built handler in the existing Go service could check a safe, configured local asset root, serve a complete local file, and otherwise contact one configured NAS origin. For an ordinary full GET, it could stream the upstream response to the browser and a temporary file, then atomically promote the file only after a successful complete transfer. Incomplete transfers must never become cache hits. The browser and rsync could share the final directory if their writes use a compatible atomic publication convention.

A simpler variant serves a local hit and redirects a miss. That may be sufficient if rsync reliably catches up and duplicate transfers are acceptable. A miss could also notify the background job to prioritize that path without delaying the browser.

### Range requests and video

Local files can be served with standard HTTP Range support. On a miss, forwarding `Range` to the NAS and relaying its `206 Partial Content`, `Content-Range`, and related headers should be possible, but a partial response must **not** be published as a complete cached file.

One conservative candidate policy is to proxy Range misses without caching them, leaving rsync to populate the full file. A full GET miss could stream and store concurrently. If video playback mostly issues Range requests, opportunistic foreground caching would then be uncommon. Possible later experiments include one deduplicated full background fetch per asset, or a chunk/range-aware cache; both add transfer or coordination complexity. These behaviors need testing with Chromium, the NAS server, seeking, large files, and interrupted Wi-Fi.

## Alternative worth considering: NFS with FS-Cache

> **Additional exploratory idea (2026-09-30).** Not implemented or selected. For this option, assume a stable NAS connection: the goal is faster access and reduced repeated asset transfers, rather than disconnected operation.

A local HTTP server could serve a read-only NFS-mounted NAS asset directory. Linux's NFS client, enabled with the `fsc` mount option and a configured CacheFiles backend (`cachefilesd`), would cache data on local disk beneath normal file reads. The HTTP server would handle Range requests and seeking; the filesystem layer would retrieve uncached file regions from the NAS and reuse cached data. This could avoid custom HTTP proxy/download logic while preserving immediate access to new content.

Background warming could be a throttled process that reads selected files through the same NFS mount and discards the output. It would use the same cache as foreground requests, without an rsync mirror or direct writes into the backend cache directory. Cache population and reuse must be verified with the chosen server and kernel: direct I/O bypasses caching, capacity and eviction affect retention, and reading part of a video does not imply that the entire file is cached.

### Eviction and policy controls

CacheFiles uses space-driven, least recently used eviction based on cached objects' access times. Objects the kernel is still using are skipped. Seldom-used data may remain indefinitely when space is plentiful; there is no fixed age-based expiry in the documented standard policy.

The main controls in `/etc/cachefilesd.conf` are:

| Setting | Behavior | Documented default |
| --- | --- | --- |
| `bcull` | Start eviction when free disk space falls below this threshold | 5% |
| `brun` | Stop eviction when free disk space rises above this threshold, provided the inode threshold is also satisfied | 7% |
| `bstop` | Stop allocating cache space below this threshold until space recovers | 1% |
| `fcull`, `frun`, `fstop` | Equivalent thresholds for free inodes | 5%, 7%, 1% respectively |

The required ordering is `bstop < bcull < brun`, with the equivalent ordering for inode thresholds. These percentages apply to the backing filesystem, rather than a maximum size of the cache directory. A dedicated cache filesystem could make the storage budget easier to bound. For example, `bcull 15%`, `brun 20%`, and `bstop 5%` would start culling below 15% free space and stop above 20%, subject to the inode thresholds.

The documented controls also include `nocull` to disable eviction. They do not offer per-folder priorities, asset pinning, or rules such as “expire after 30 days.” Warming every file repeatedly could make otherwise unused assets appear recent; selectively warming upcoming or changed assets may cooperate better with eviction.

### Validation before choosing this option

Measure cold and warm video playback, seeking, cache reuse after reboot with the NAS available, and behavior when assets change on the NAS. Confirm disk-cache hits using filesystem statistics and network traffic, rather than mistaking RAM-cache hits for persistent caching. Test the chosen HTTP server's actual file-reading path and the interaction between warming, foreground transfers, and cache pressure.

NFS may still contact the NAS for metadata and coherence checks even when file data is cached. That is acceptable under the stable-connection assumption; this option does not promise offline playback.

References: [NFS mount options and coherence](https://man7.org/linux/man-pages/man5/nfs.5.html), [CacheFiles configuration and culling](https://docs.kernel.org/filesystems/caching/cachefiles.html), and [cachefilesd configuration manual](https://kernel.googlesource.com/pub/scm/linux/kernel/git/dhowells/cachefilesd/+/refs/heads/master/cachefilesd.conf.5). Validate configuration against the appliance's actual kernel and package versions.

## Questions before implementation

- How are asset URLs mapped to NAS paths, and how are traversal, redirects, and untrusted upstream URLs excluded?
- How does the client know that a local copy is current when the NAS replaces an asset under the same path? Versioned names or a manifest/hash may make this explicit.
- How do rsync and foreground fetches publish complete files atomically, avoid competing writes, and recover from interrupted transfers?
- Should the mirror include all assets, a selected set, or only recently relevant files? What disk limit and eviction policy apply?
- What happens to a cached asset when Wi-Fi or the NAS is unavailable, and what happens on a miss?
- Does Chromium's actual video request pattern justify active caching beyond the simple redirect plus rsync design?

Start with measured browser/NAS Range behavior and the simplest scheme that meets playback needs. This note records options, not an implementation commitment.
