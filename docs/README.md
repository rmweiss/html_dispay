# HTML Display documentation

HTML Display is a small Go control server for an existing Chromium kiosk tab. It accepts a URL or a complete HTML document over the local network and replaces the visible page.

- [Using the display](usage.md) — endpoints, examples, and current behavior
- [Kiosk deployment](deployment.md) — the tested NixOS setup and what must be supplied outside this repository
- [Audio and Scene Composer](audio.md) — the planned scene/system boundary and three-class audio priority policy
- [Temporary mpv takeover: first concept](mpv-takeover-concept.md) — proposed mpv playback and return to Chromium; not implemented
- [Plans and open questions](roadmap.md) — desired behavior that the current Go code does not implement

The [repository README](../README.md) is the quick start. These pages describe the current code unless a section explicitly says **Planned**. The project-wide [HTML Display design document](https://docs.google.com/document/d/1A4dgv5Gr0XnCopPfRQsLVq9IHdqAgYk8Yn513VgB0mo/edit) is the authoritative design reference; these pages detail the repository state and proposed implementation.
