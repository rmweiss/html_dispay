# Plans and open questions

This page separates desired behavior in the earlier design notes from features in the current Go repository. None of the items below should be assumed to work today.

| Desired behavior | Current state | Decision for implementation |
| --- | --- | --- |
| Restore the previous display after a service or machine restart | Only the uploaded HTML file persists; the active mode and last URL do not | Define persisted mode/URL metadata and startup navigation, including what to do when the URL is unavailable |
| MQTT as a second control protocol beside REST | Only REST control is implemented | Add MQTT as a protocol adapter over the same internal display/state operations as REST, rather than implementing separate behavior |
| Selectively persist display state through MQTT | No MQTT state is published or restored | Treat persistence as a property of selected canonical state. For example, when the displayed URL changes through REST, MQTT, or another future input, publish the resulting URL to a retained MQTT state topic so it can be restored after reconnect/restart. Transient actions such as reload or one-shot playback must not be retained |
| Home Assistant MQTT discovery | No Home Assistant discovery is implemented | Publish discovery metadata for useful display entities when MQTT connects and republish it periodically (for example every few minutes). Current entity state can be republished at the same time so integration state self-heals after broker or Home Assistant restarts |
| Optional fallback for an unreachable external URL | The local `/display/content` route already shows an HTML status page when no upload exists or the stored file cannot be read; `/` shows an idle/status page. A failed CDP URL navigation returns an API error but does not intentionally navigate to a local fallback | Decide whether external URL connection failures should also replace the kiosk page with a local status screen; no automatic retry is currently specified |
| Play Scene Composer audio on the kiosk | Browser audio worked in the reported appliance setup; Scene Composer's mixer and trigger API are implemented | Validate kiosk playback with the [Scene Composer audio design](https://github.com/rmweiss/scene_composer/blob/main/docs/audio.md) in that project; see [audio integration](audio.md) |
| Give external audio lower priority than browser audio, with mpv takeover highest | Only browser audio is reported working; the policy, external player, and mpv are not configured | Test the [three-class audio policy](audio.md) with WirePlumber and Squeezelite on the N100; keep external-player transport outside HTML Display and validate [mpv takeover](mpv-takeover-concept.md) separately |

## MQTT state concept

MQTT should complement REST, not replace it. Both protocols should feed the same internal command/state layer so that changing the display through either protocol has identical behavior.

The important distinction is between commands and canonical state:

- command/request topics are normally non-retained;
- selected state topics may be retained when that state is intended to survive restart;
- a state change should be published regardless of which protocol caused it;
- receiving retained state at startup may be used to restore the display;
- idempotent state handling should avoid unnecessary reload/publish loops when the restored value is already active.

A likely URL pattern is a non-retained request topic plus a retained state topic, for example:

```text
html-display/<id>/display/url/set
html-display/<id>/display/url/state
```

If a REST request changes the displayed URL, the server should update the canonical state and publish the resulting URL to `.../state` with retain enabled. The same should happen for an MQTT-originated change. This makes persistence independent of the protocol that initiated the change.

Only a small whitelist of state should be persistent. Candidate examples include the displayed URL, display enabled/disabled state, and volume if those features exist. Transient actions such as reload, play-once commands, notifications, or future effect triggers should not be retained.

For Home Assistant integration, the server may publish MQTT Discovery configuration for the display and its entities. Discovery should be emitted when MQTT connects and may also be republished periodically, together with current entity state, so Home Assistant can recover cleanly after restarts or missed discovery messages.

The earlier notes mention a local page's SSE connection. That belonged to a superseded iframe design. The current HTML Display implementation navigates the kiosk tab directly and exposes no SSE endpoint. Scene Composer's separate player channel for scene-instance updates and effect commands does not change that.

The product intent remains a display for locally generated, trusted HTML and occasional trusted local web applications. Arbitrary third-party browsing and casting are outside the stated scope. The proposed monitor resolution and the proposed audio policy should be validated against the actual appliance before becoming implementation requirements.
