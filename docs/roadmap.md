# Plans and open questions

This page separates desired behavior in the earlier design notes from features in the current Go repository. None of the items below should be assumed to work today.

| Desired behavior | Current state | Decision for implementation |
| --- | --- | --- |
| Restore the previous display after a service or machine restart | Only the uploaded HTML file persists; the active mode and last URL do not | Define persisted mode/URL metadata and startup navigation, including what to do when the URL is unavailable |
| Show a local “Page unavailable” screen on connection failure or timeout | The URL API reports navigation errors; no fallback page or retry logic exists | Decide which network and browser failures trigger fallback and whether any retry is wanted |
| Send an audio file directly to the display | No audio upload endpoint or player is implemented | Decide whether browser HTML audio is sufficient or a separate playback command is needed |
| Provide a reproducible NixOS appliance configuration | No NixOS files are in this repository | Bring the tested configuration into version control after reviewing machine-specific and sensitive settings |

The earlier notes mention a local page's SSE connection. That belonged to a superseded iframe design. The current implementation navigates the kiosk tab directly and exposes no SSE endpoint, so SSE is not part of this roadmap.

The product intent remains a display for locally generated, trusted HTML and occasional trusted local web applications. Arbitrary third-party browsing and casting are outside the stated scope. The proposed monitor resolution and any future audio API should be validated against the actual appliance before becoming implementation requirements.
