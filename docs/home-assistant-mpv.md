# Home Assistant MPV media-player integration (no custom components)

The standalone Go MPV controller already publishes Home Assistant MQTT Discovery
**sensors** for `playback`, `source`, `position`, `duration`, buffer metrics,
`video_active`, and (after the separate lifecycle-fix PR) `error`.
Its MQTT commands are at `html-display/mpv/+/set`.

Home Assistant's built-in MQTT integration does **not** implement MQTT
`media_player` discovery. Do not publish
`homeassistant/media_player/.../config` expecting a native entity.

Home Assistant 2026.11 supports a built-in **Template media player**, including
play media, pause, resume, seek, stop, title/source, position, duration, and
buffering. This bridges the existing MQTT state and command topics without
installing a HACS/custom integration or changing the controller.

## Suggested configuration

Copy the snippet below into your Home Assistant `configuration.yaml` and merge
it with any existing `template:` section (do not create two top-level
`template:` sections). Check the actual entity IDs first under **Developer
Tools → States**: MQTT Discovery and your HA entity registry may have renamed
them. Replace the example sensor IDs below if necessary.

```yaml
template:
  - media_player:
      - name: "HTML Display MPV"
        unique_id: html_display_mpv_template_player
        device_class: tv

        availability: >
          {{ states('sensor.html_display_mpv_playback')
             not in ['unknown', 'unavailable'] }}

        # A template media player accepts idle, playing, paused and buffering,
        # but not the Go controller's starting, stopping or error states.
        state: >
          {% set s = states('sensor.html_display_mpv_playback') %}
          {% if s in ['starting', 'buffering'] %}
            buffering
          {% elif s in ['playing', 'paused'] %}
            {{ s }}
          {% else %}
            idle
          {% endif %}

        media_content_id: "{{ states('sensor.html_display_mpv_source') }}"
        media_content_type: "video"
        media_title: "{{ states('sensor.html_display_mpv_source') }}"
        media_duration: "{{ states('sensor.html_display_mpv_duration') | float(0) | int }}"
        media_position: "{{ states('sensor.html_display_mpv_position') | float(0) | int }}"

        media_play:
          action: mqtt.publish
          data:
            topic: html-display/mpv/playback/set
            payload: resume
            retain: false
        media_pause:
          action: mqtt.publish
          data:
            topic: html-display/mpv/playback/set
            payload: pause
            retain: false
        media_stop:
          action: mqtt.publish
          data:
            topic: html-display/mpv/playback/set
            payload: stop
            retain: false
        media_seek:
          action: mqtt.publish
          data:
            topic: html-display/mpv/seek/set
            payload: "{{ seek_position }}"
            retain: false
        play_media:
          action: mqtt.publish
          data:
            topic: html-display/mpv/source/set
            payload: "{{ media_content_id }}"
            retain: false
```

**Important behavior:** `media_play` **resumes** a paused video; it doesn't
restart an earlier video when the controller is idle. Use Home Assistant's
`media_player.play_media` action with a URL/path (or the existing REST endpoint)
to start a new video. If a video is already playing, the controller refuses a
second play request (no auto-replace).

Apply the YAML using Home Assistant's configuration validation and restart.
Verify `media_player.html_display_mpv` appears, follows playing/paused/buffering/
idle, and controls MPV through MQTT. This sample was checked against the
Home Assistant 2026.11 documented template media-player schema but has **not
yet been tested** on your HA installation.

Source and progress are taken from the controller's already-discovered MQTT
sensors; the source is a URL/path, **not extracted video metadata**.
`media_position` is polled in seconds. Volume is deliberately absent: HDMI
volume remains a separate TV control and no MPV gain control is currently
exposed. Brightness also remains owned by Home Assistant and can use the
existing `binary_sensor.html_display_mpv_video_active` to apply a temporary
boost while video plays.

## Future options

If you prefer a fully auto-discovered `media_player` with no HA YAML, that
requires choosing and installing a third-party MQTT media-player integration
or implementing a dedicated native Home Assistant integration. It cannot be
achieved solely by publishing a standard MQTT discovery payload.

Official documentation:
- https://www.home-assistant.io/integrations/template/#media-player
- https://www.home-assistant.io/integrations/mqtt/
