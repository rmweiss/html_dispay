# Scene Composer layer motion: Ken Burns concept

> **Status: concept / possible future implementation.**
>
> This is not implemented by the current HTML Display server. It describes a possible Scene Composer layer feature.

## Idea

Image and video layers could optionally use a slow pan-and-zoom treatment similar to the Ken Burns effect. The goal is to add subtle motion to otherwise static or gently looping scene elements without requiring pre-rendered video.

Rather than implementing this as a one-off special effect, it may be better to treat it as a preset on top of a more general per-layer transform animation system.

## Possible layer model

A layer could define a start transform, an end transform, a duration, and easing:

```json
{
  "kenBurns": {
    "enabled": true,
    "duration": 30,
    "start": {
      "x": 50,
      "y": 50,
      "scale": 1.0
    },
    "end": {
      "x": 42,
      "y": 55,
      "scale": 1.15
    },
    "easing": "ease-in-out",
    "alternate": true
  }
}
```

The exact schema is intentionally undecided.

Useful parameters would likely include:

- start and end position;
- start and end scale;
- duration;
- easing;
- repeat behavior;
- whether the animation alternates direction;
- optional presets such as zoom in, zoom out, pan left/right, or slow drift.

## Rendering approach

For both image and video layers, the media element could sit inside a fixed layer container with `overflow: hidden`. The animation would be applied to the inner media element using CSS transforms such as `translate(...)` and `scale(...)`.

Keeping the transform on the inner element avoids changing the layer's layout and prevents a zoomed image or video from spilling outside the layer bounds.

For video layers, transform animation should remain independent of video playback. A looping cloud, rain, fire, or similar video could therefore continue playing normally while the entire visual slowly pans or zooms.

## Generalizing beyond Ken Burns

A reusable transform animation model could support more than the classic Ken Burns effect. The same mechanism could later provide:

- slow drifting overlays;
- floating foreground elements;
- subtle parallax-like movement;
- continuously moving texture layers;
- manually defined start/end transforms.

In that model, "Ken Burns" would mainly be a convenient preset or UI mode for configuring a transform animation rather than a separate rendering subsystem.

## UI possibility

A future editor could expose a few simple presets first:

- Zoom in
- Zoom out
- Pan left
- Pan right
- Slow drift

Advanced controls could then expose start/end position, scale, duration, easing, and repeat behavior.

The intent is to keep the common case simple while leaving room for more expressive motion later.
