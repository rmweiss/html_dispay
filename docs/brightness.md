# Samsung Frame backlight control and deep-standby recovery

Status: field observations on Samsung The Frame LS03B (2022), November 2026. This page distinguishes tested behavior from working hypotheses. See [ExLink command reference](exlink.md) for byte-level protocol details.

## Brightness terminology

Three different controls must not be conflated:

- **Backlight** is ExLink query `F0 0A` and write `0B 01 00 VV`; valid configured range **0–50** on this TV. This changes the intended LED illumination, not browser content brightness. The Go `tvctl backlight` command and Home Assistant backlight number entity use it.
- **Picture Brightness** (black-level-related picture setting) is ExLink query `F0 0C` and write `0B 03 00 VV`. It is not the LED backlight setting.
- **Contrast** is query `F0 0B`. Changing contrast or color intensity may alter the image while the physical backlight remains stuck.

A readback of the configured backlight value **does not prove that the panel has actually applied it**.

## Verified behavior

After a short TV standby the picture returns almost immediately and backlight changes normally take effect. After approximately **15–20 minutes off**, a tested wake showed a “Powered by Knox” boot screen and an HDMI input overlay; the backlight was then *frozen* at a fixed visible brightness.

While affected:
- Changing backlight with the Samsung remote or ExLink changed the **reported setting**, but **not the visible illumination**.
- Other picture controls, including contrast and color intensity, still worked.
- Changing picture mode did **not** restore backlight control.
- Resetting all picture settings did **not** restore it.
- A brief TV off/on using the Samsung remote restored backlight control.
- A brief **ExLink power off/on also restored backlight control** (confirmed on the actual appliance).
- Before and after the successful ExLink recovery cycle, the **64 queried ExLink status identifiers 0x00–0x3F had identical values and raw responses**. No status register in that range exposed the failure.

This is consistent with a TV-side deep-wake initialization issue affecting the backlight-control path, but the precise firmware/hardware cause is **unknown**. No direct LED PWM, actual light-output, or internal driver-state measurement was made. Do not describe this as a proven Samsung firmware defect.

## Recovery

Once the TV has finished waking, the following existing `tvctl` commands were confirmed to repair the frozen-brightness condition:

```sh
tvctl power off
sleep 3
tvctl power on
```

The three-second wait is the tested example, not a guaranteed minimum or a recommendation to repeatedly power-cycle a faulty unit. Reapply command-result OSD suppression *after* recovery if needed:

```sh
tvctl osd off
```

The OSD command itself can produce a visible notification; avoid continuous reapplication.

## Planned Home Assistant workaround (not yet implemented or validated)

Detect a long period off, allow the first wake to finish, briefly cycle TV power once, then restore OSD OFF. A **10-minute threshold** is a conservative starting point given the observed failure after approximately 15–20 minutes; it is **not a measured deep-standby transition threshold**. Suggested startup delays are provisional.

Prefer storing the actual OFF-command timestamp in an `input_datetime` helper if Home Assistant's MQTT power entity does not reliably report standby. Home Assistant's `last_changed` for an entity can reset when the entity is recreated after HA restart; an intervening `unavailable` state can also break a strict `off` → `on` trigger. A correctly configured timestamp helper can preserve the original OFF time across HA restarts. Ensure the recovery cycle does not retrigger itself and do not issue a second cycle while one is in progress.

**Do not infer frozen brightness from backlight readback alone:** the TV reports normal settings even while visibly stuck.

## Diagnostic procedure

If the fault returns:
1. Observe whether “Powered by Knox” appears (versus instantaneous wake).
2. Before cycling power, capture read-only status: `tvctl status` and/or `tvctl raw F0 0A 00 00`.
3. Change backlight between two safe values; compare physical illumination with readback.
4. Run one short ExLink power cycle; verify brightness changes take effect again.
5. If needed, repeat read-only status queries. The previous 0x00–0x3F scan showed no changed status bits before/after recovery.

The prior full 0x00–0xFF `F0` query scan tested **OSD ON versus OFF**, not the brightness fault. It found no OSD-dependent status values. These two experiments should not be conflated.

## Open questions

- What exact standby duration or software event triggers the full Knox startup?
- Does the command-result OSD setting reset on every full wake or only some?
- Can another less disruptive TV-side operation reinitialize physical backlight control?
- Could an external lux or optical sensor detect the stuck illumination? Ambient-light-based automatic adjustment is deferred until a sensor is suitably positioned.

The Go TV daemon remains independent of Chromium/Sway. A Home Assistant recovery policy is preferable to embedding speculative fault detection in `tvctl`.
