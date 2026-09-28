# Plans and open questions

This page separates desired behavior in the earlier design notes from features in the current Go repository. None of the items below should be assumed to work today.

| Desired behavior | Current state | Decision for implementation |
| --- | --- | --- |
| Restore the previous display after a service or machine restart | Only the uploaded HTML file persists; the active mode and last URL do not | Define persisted mode/URL metadata and startup navigation, including what to do when the URL is unavailable |
| Show a local “Page unavailable” screen on connection failure or timeout | The URL API reports navigation errors; no fallback page or retry logic exists | Decide which network and browser failures trigger fallback and whether any retry is wanted |
| Play Scene Composer audio on the kiosk | Browser audio worked in the reported appliance setup; Scene Composer's mixer and trigger API are planned | Implement and test the [Scene Composer audio design](https://github.com/rmweiss/scene_composer/blob/main/docs/audio.md) in that project; see [audio integration](audio.md) |
| Optionally play audio independent of Scene Composer | No independent audio command or player is implemented | Revisit if spontaneous sounds or background music alongside another page become a concrete need; define navigation, stop, storage, volume, and concurrency behavior then |
| Provide a reproducible NixOS appliance configuration | No NixOS files are in this repository | Bring the tested configuration into version control after reviewing machine-specific and sensitive settings |

The earlier notes mention a local page's SSE connection. That belonged to a superseded iframe design. The current HTML Display implementation navigates the kiosk tab directly and exposes no SSE endpoint. Scene Composer's separate player channel for scene-instance updates and effect commands does not change that.

The product intent remains a display for locally generated, trusted HTML and occasional trusted local web applications. Arbitrary third-party browsing and casting are outside the stated scope. The proposed monitor resolution and any independent audio command should be validated against the actual appliance before becoming implementation requirements.
