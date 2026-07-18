# Throbber UI prototype

**Question:** What should the per-phase wait throbber look like in the terminal?

**Run:** `go run ./cmd/ship throbber` (Ctrl-C to stop early)

**Direction:** Full-screen living tableau (gnhf-scale presence), not a one-line spinner.

| Choice | Decision |
|--------|----------|
| Shape | Large living tableau |
| World | Vertical rocket ascending into night sky / space |
| Screen | Full-screen takeover |
| Status | Bottom HUD (phase + elapsed) |
| Motion | Twinkling + scrolling stars, rocket bob, downward exhaust plume |
| Palette | Deep space indigo + silver hull + amber thrusters |

**Feedback round 2:** Ship felt too small → enlarged tall-ship silhouette.

**Feedback round 3:** Hull too similar to waves; captain removed. Warm wood/cream hull against indigo sea.

**Feedback round 4:** Hull looked upside down → flipped to wide deck / narrow keel.

**Feedback round 5:** Ship a little smaller (shorter sails, ~51-wide hull).

**Feedback round 6:** Swap sailing boat for spaceship (side view over sea) — rejected.

**Feedback round 7:** Vertical rocket ascending into space (nose up, plume down). No water; full starfield sky.

**Feedback round 8:** Exhaust too static/big → shorter breathing plume with flicker and tip crackle.

**Bugfix:** Plume panic `index out of range [-1]` — Go signed `%` with `dx < 0` into flicker slice. Removed dead flicker index; tip spark uses positive modulo.

**Feedback round 9:** Still noisy / too big → compact 5-line rocket, sparse quiet stars, short single-column plume, calmer HUD.

**Feedback round 10:** Twin rockets — mis-hear; wanted second exhaust, not second ship.

**Feedback round 11:** One rocket again; twin exhaust under left + right engine nozzles.

**Feedback round 12:** More stars (denser field, still quiet twinkle).

**Verdict:** Prototype accepted visually; still throwaway under `internal/throbber/prototype`. Needs cleanup + Run-loop integration later.
