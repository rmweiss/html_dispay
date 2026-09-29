# Plans and open questions

This page separates desired behavior in the earlier design notes from features in the current Go repository. None of the items below should be assumed to work today.

| Desired behavior | Current state | Decision for implementation |
| --- | --- | --- |
| Restore the previous display after a service or machine restart | Only the uploaded HTML file persists; the active mode and last URL do not | Define persisted mode/URL metadata and startup navigation, including what to do when the URL is unavailable |
| Optional fallback for an unreachable external URL | The local `/display/content` route already shows an HTML status page when no upload exists or the stored file cannot be read; `/` shows an idle/status page. A failed CDP URL navigation returns an API error but does not intentionally navigate to a local fallback | Decide whether external URL connection failures should also replace the kiosk page with a local status screen; no automatic retry is currently specified |
| Play Scene Composer audio on the kiosk | Browser audio worked in the reported appliance setup; Scene Composer's mixer and trigger API are planned | Implement and test the [Scene Composer audio design](https://github.com/rmweiss/scene_composer/blob/main/docs/audio.md) in that project; see [audio integration](audio.md) |
| Give external audio lower priority than browser audio, with mpv takeover highest | Only browser audio is reported working; the policy, external player, and mpv are not configured | Test the [three-class audio policy](audio.md) with WirePlumber and Squeezelite on the N100; keep external-player transport outside HTML Display and validate [mpv takeover](mpv-takeover-concept.md) separately |

The earlier notes mention a local page's SSE connection. That belonged to a superseded iframe design. The current HTML Display implementation navigates the kiosk tab directly and exposes no SSE endpoint. Scene Composer's separate player channel for scene-instance updates and effect commands does not change that.

The product intent remains a display for locally generated, trusted HTML and occasional trusted local web applications. Arbitrary third-party browsing and casting are outside the stated scope. The proposed monitor resolution and the proposed audio policy should be validated against the actual appliance before becoming implementation requirements.
