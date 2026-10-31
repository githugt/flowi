# 0006: Fyne for the whole GUI, pending the canvas prototype

> **Superseded.** The GUI is now Wails 3 with a Svelte front end (the `flowi` repository). The Fyne app was retired on 2026-10-23.

- **Status:** Accepted (G0, 2026-10-08)
- **Date:** 2026-10-06

## Context

The GUI is mostly standard widgets (trees, tabs, forms, tables, dialogs), plus one custom
part: a flow canvas with draggable groups and straight and curved edges. Fyne has rich
widgets, a stable v2 API and a large community, but no node-graph widget. Gio suits custom
vector drawing better but provides fewer ready-made widgets and is pre-1.0.

## Decision

- The GUI uses **one** toolkit; Fyne and Gio are never mixed.
- Fyne is the default. A throwaway Fyne canvas prototype (milestone G0) must show smooth
  interaction at 100+ nodes, with anti-aliased curves, before real GUI work begins.
- If Fyne clearly fails that bar, the entire GUI moves to Gio. If the result is unclear,
  the more widely used and extensible toolkit, Fyne, is chosen.

## Consequences

- G0 must finish before the GUI shell milestone starts, so a switch only discards the prototype.

## G0 result

**The prototype:**
- A throwaway Fyne v2.8.1 app with 126 and 258 units: request blocks with hook chips, action strips, and fork lanes.
- Edges are drawn with `x/image/vector`.
- It handles pan, zoom, unit and selection drag, live edge re-routing, and the insertion highlight.
- A fake 1,000 events/s status stream is coalesced to 30 UI updates/s.
- It ran on an M-series Mac with a retina display and a 1223×776 dp window.

**How it was measured:** each scripted step's time includes one paint plus a full GL read-back (`Capture`). That adds about 3 ms, so the numbers are pessimistic.

| Phase (126 → 258 units) | p50 | p95 |
|---|---|---|
| Pan | 3.1 ms | 12–13 ms |
| Drag one unit (live re-route, insertion highlight) | 3.2 ms | 13–14 ms |
| Drag a 10-unit selection | 3.1 ms | 13 ms |
| Continuous zoom sweep 0.3×→2× (60 steps) | 13 ms | 25–30 ms |

**Outcome:**
- **Fyne for the whole GUI.** Pan and drag stay well above 50 fps at 126 and at 258 units.
- **Quality:** curves are anti-aliased at every zoom, and labels are crisp at 1× and 2×.
- **Zoom:** a continuous zoom sweep, the worst case, runs at about 35–40 fps.

**Findings for the production canvas (G2):**
- **The edge layer is the hot spot.**
  - Each edge must be rasterized inside its own bounding box. A full-window rasterizer cost about 70 ms per frame at retina sizes; bounded, it is 2–4 ms.
  - Fyne re-uploads a `canvas.Raster` as a new texture on every refresh. One window-sized edge raster therefore costs a full-window upload each frame.
  - One raster *per edge* makes pan free (objects only move) and lowers drag p95 to 8–10 ms, but makes zoom slower (one texture per edge).
  - G2 should take per-edge rasters, or tiles, if profiling of real flows agrees. That refines design §10.3's "one raster layer".
- **Zoom cost must follow what is visible.** Re-laying out off-screen units made a zoom step cost the whole flow (44 ms p95 at 258 units). Resizing only visible units, and the rest when they scroll in, made zoom independent of flow size.
- **Hide labels below about 0.35×.** They are unreadable there anyway, and this saves glyph work.
- **Threading:**
  - `Canvas().Capture()` must run on the UI goroutine. From another goroutine it races the GL context and crashes (SIGSEGV in `glClear`).
  - `Capture()` returns the last painted frame, not a fresh render.

Interactive feel was judged from the scripted numbers. A hands-on check (`go run .` in the prototype) is still worth doing before G2.
