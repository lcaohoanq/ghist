---
baseline_schema: "2.0"
pack: "readme-visuals"
document: "introduction"
status: "active"
updated: "2026-10-12"
code_ref: "uncommitted"
---

# README visuals - introduction

## Scope

The visual identity of the ghist README: a hero banner, a workflow figure, four explainer figures (renames, read-only, PR and MR lookup, code layers), and the README text rewritten around them. Out of scope: the Go code, a landing page, a social preview image, a license, release packaging.

## Current truth

All work is in the working tree on top of `3d2a829`; nothing is committed.

| Statement | Status | Source |
|---|---|---|
| README rewritten: hero in the H1, pitch sentence, original screenshot kept, workflow figure with a text form, renames and read-only figures, then installation (with the PR and MR figure), usage, development (with the layers figure) | implemented | `README.md` |
| All earlier README content is still present; the key table moved into `<details>` | implemented | `README.md` |
| Six figures rendered, one light and one dark image each | implemented | `assets/*.webp` |
| Workflow branches match the key handler | verified against source, not run | `internal/tui/model.go` `key()` |
| Arrows in the layers figure match package imports | verified by import scan | `cmd/ghist`, `internal/*` imports |
| PR and MR figure matches the lookup code | verified against source, not run | `internal/hosting/hosting.go` |
| Read-only figure: Git commands match `internal/git` | verified against source | `internal/git/*.go` |
| Read-only figure: commits made after opening stay out | unverified live | RV-Q3 in `hallucination` |
| `o` opens the first PR or MR in a browser | unverified live | README Usage text |
| README renders correctly on github.com, light, dark, phone | unverified | - |
| `<picture>` inside `<h1>` and inside `<details>` renders on github.com | unverified | - |
| Go tests pass | not run, no `go` on PATH, no Go code touched | - |

Image files and sizes:

- `assets/hero.webp` 171 KB lossless, `assets/hero-dark.webp` 283 KB lossy (see RV-D7 in `hallucination`).
- `assets/workflow.webp` 79 KB, `assets/workflow-dark.webp` 78 KB, both lossless.
- `assets/renames.webp` 70 KB and `assets/renames-dark.webp` 69 KB, `assets/readonly.webp` 61 KB and `assets/readonly-dark.webp` 61 KB, `assets/reviews.webp` 52 KB and `assets/reviews-dark.webp` 51 KB, `assets/layers.webp` 49 KB and `assets/layers-dark.webp` 49 KB, all lossless.
- All are 2400 px wide: 2400x1120 for the hero pair, 2400x1280 for the workflow pair, 2400x1080 for `renames`, 2400x1240 for `readonly` and `layers`, 2400x1000 for `reviews`.

Measured contrast, smallest text pairs: ink on yellow field 13.05, ink on card 18.51, off-white on violet field 13.07, card ink on off-white 16.40, terminal text 8.10, terminal muted text 5.69 on the terminal ground and 4.74 on the added-line tint.

## Target

A README whose first screen answers what ghist is and why, shows the real interface, shows how a visit goes from History to Diff to File, and answers the next questions a reader has (what happens on a rename, whether it touches the repository, how the PR and MR link works, how the code is organized), with every picture truthful to the code, in light and dark, each image at or under 300 KB.

## Constraints

- GitHub README markup only: `<picture>`, `<img>`, `<details>`, tables, fenced code. No CSS, no JavaScript.
- Each image at or under 300 KB; every re-render adds its size to git history again.
- A picture states only what the README or the code states. No counts in images.
- Install and run commands stay as text and are never inside `<details>`.
- ASCII hyphen only in the README and in image sources.
- Fonts load from Google Fonts, so rendering needs network; rendering uses Edge or Chrome already on the machine through Playwright run with `uvx`.

## Signature

| Part | Choice |
|---|---|
| Concept | ghist answers "How did this code get here?", so the pictures stage the real interface and its real keys like a product poster |
| Shape language | flat colour field, 3px ink edges, hard offset shadows, keycaps with a thick lower edge, the real terminal drawn in vector |
| Typeface | Bricolage Grotesque (display, condensed and heavy) with JetBrains Mono for keys and terminal text |
| Palette logic | one saturated field per theme (yellow light, deep violet dark), ink for every edge and shadow, terminal in the Tokyo Night colours of the README screenshot |
| Presentation | hero as a poster: wordmark, pitch quote, tilted window, "read only" sticker; workflow as three cards joined by keycap arrows; explainers as cards, chips and dashed outlines for what is optional or not shown |
| Not | soft gradients, stock icons, anything the code does not do |

## Reading order

`readme-visuals.roadmap.md` for status and the next action, `readme-visuals.hallucination.md` for why each choice was made, `readme-visuals.sourcecode.md` for how the images are built.
