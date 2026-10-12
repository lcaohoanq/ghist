---
baseline_schema: "2.0"
pack: "readme-visuals"
document: "roadmap"
status: "active"
updated: "2026-10-12"
code_ref: "uncommitted"
---

# README visuals - roadmap

| Phase | Status | Gate |
|---|---|---|
| Phase 1 (RV-P1) Assets | complete | four images rendered, sized, contrast measured |
| Phase 2 (RV-P2) README | complete | all earlier content kept, every image path resolves |
| Phase 3 (RV-P3) Review and publish | pending | owner review, then commit |

## RV-P1: Phase 1 - Assets

- Depends on: nothing.
- Status: complete.
- Acceptance: hero, workflow and the four explainer figures rendered in light and dark; each at or under 300 KB; smallest text pairs at or above 4.5; text identical in meaning to the README.
- Evidence:
  - `render.py` printed `hero.webp` 171 KB, `workflow.webp` 79 KB, `workflow-dark.webp` 78 KB, and `hero-dark.webp` 318 to 322 KB lossless with a WARN over 300 KB.
  - `hero-dark.webp` re-encoded lossy at quality 94 to 283 KB (RV-D7 in `hallucination`).
  - `contrast.py` on the smallest pairs: all pass; terminal muted text fixed from 2.76 to 5.69 (RV-D5).
  - Each of the four images was opened and looked at.
  - `renames`, `readonly`, `reviews` and `layers` rendered with `--items`, so the lossy `hero-dark.webp` was left alone; each pair is 48 to 70 KB lossless with no WARN, and the light and dark images of `readonly` and `renames` were looked at, the others in light.
  - Claims in the four figures were read against `internal/git`, `internal/hosting/hosting.go`, package imports and `docs/plan.md` (RV-D12, RV-D13, RV-D14 in `hallucination`).

## RV-P2: Phase 2 - README

- Depends on: Phase 1.
- Status: complete.
- Acceptance: hero first, one bold sentence, screenshot, workflow figure with text form, then install, usage, development; no earlier content lost; no en or em dash.
- Evidence:
  - `README.md` rewritten; the four `assets/` paths exist; dash check printed 0 in `README.md`, `figures.html`, `shared.css`.
  - Branch claims in the workflow figure read against `internal/tui/model.go` (RV-D10).
  - Four figures placed in the README as planned (RV-D11): all twelve `assets/*.webp` paths in `README.md` exist; dash check printed 0 again in `README.md`, `figures.html`, `shared.css`.
- Not observed: the README on github.com, `<picture>` inside `<h1>` and `<details>` (RV-Q1), commits made after opening staying out of History (RV-Q3), Go tests (no `go` on PATH, no Go code touched).

## RV-P3: Phase 3 - Review and publish

- Depends on: Phase 2.
- Status: pending.
- Acceptance: owner has looked at the twelve `assets/*.webp` images and `README.md`; RV-Q3 observed on a scratch repository; requested changes applied by editing `assets/source/` and re-rendering; changes committed (RV-Q2); README checked on github.com (RV-Q1).
- Risk: each re-render adds image size to git history again, so settle the design before the commit.
- Risk: `hero-dark.webp` needs the lossy re-encode after every render (RV-D7); render a subset with `--items` to avoid touching it.

## Risks

| Risk | Effect | Handled by |
|---|---|---|
| Re-render without the lossy step | dark hero at about 320 KB | command in `figures.html` header |
| Google Fonts unreachable at render | fallback face in images | `render.py` WARN, render online |
| Interface changes later | hero and workflow drift from the product | re-check RV-D10 against `internal/tui/model.go` |

## Next action

Owner: open the images in `assets/` (hero, workflow, renames, readonly, reviews, layers, each light and dark) and `README.md` in the editor and say either "keep" or the exact changes wanted. After "keep", ask for explicit authorization to commit, then follow RV-Q2.
