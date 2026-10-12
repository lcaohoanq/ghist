---
baseline_schema: "2.0"
pack: "readme-visuals"
document: "hallucination"
status: "active"
updated: "2026-10-12"
code_ref: "uncommitted"
---

# README visuals - hallucination

## Entry index

| ID | About | Status | Related |
|---|---|---|---|
| RV-D1 | Subject of every picture | closed | RV-D10 |
| RV-D2 | Hero headline | closed | RV-D3 |
| RV-D3 | What the hero may contain | closed | RV-D2 |
| RV-D4 | Example data in the terminal | closed | RV-D1 |
| RV-D5 | Terminal palette and muted colour | closed | - |
| RV-D6 | Typeface pairing | closed | - |
| RV-D7 | Dark hero is lossy | closed | RV-D8 |
| RV-D8 | Where sources and images live | closed | RV-D7 |
| RV-D9 | What the README keeps | closed | - |
| RV-D10 | Workflow figure branches | closed | RV-D1 |
| RV-D11 | Explainer figures and their README placement | closed | RV-D9 |
| RV-D12 | Rename figure examples and caveat | closed | RV-D1 |
| RV-D13 | Basis of the read-only figure | closed | RV-Q3 |
| RV-D14 | Meaning of arrows in the layers figure | closed | RV-D1 |
| RV-Q1 | github.com rendering | open | - |
| RV-Q2 | Commit grouping and authorization | open | - |
| RV-Q3 | Later commits excluded while ghist is open | open | RV-D13 |

## Closed decisions

### RV-D1: Subject of every picture

- Decision: every picture shows the real ghist interface and its real keys.
- Why: a figure is where an invented claim hides, and the interface is the product's strongest asset.
- Breaks if ignored: a picture drifts from what the code does and the README contradicts itself.

### RV-D2: Hero headline

- Decision: the headline is the project's own question, "How did this code get here?" from `docs/plan.md` ("The goal is to make 'How did this code get here?' cheap to answer"), not a usage instruction.
- Why: the owner wants the slogan to be the pitch, and the workflow figure already carries usage.
- Breaks if ignored: the hero turns into a how-to and repeats the workflow figure.
- Rejected: a usage line as the slogan - owner rejected it.

### RV-D3: What the hero may contain

- Decision: the hero holds the wordmark, the question, one sentence, a command chip (`$ ghist internal/hosting/hosting.go`), the tilted window and the "read only" sticker; keycaps live only in the workflow figure.
- Why: keys are usage, and usage belongs to the figure built for it (RV-D2).
- Breaks if ignored: the keycap row returns to the hero and the pitch is diluted.

### RV-D4: Example data in the terminal

- Decision: the terminal shows the real history of `internal/hosting/hosting.go` in this repository (`git log --follow`, six versions, commit `34fd8a5` selected), author names unmodified.
- Why: real data cannot be mistaken for a claim the product does not back; the owner approved keeping the names.
- Breaks if ignored: invented commits or names in an image read as a real project record.

### RV-D5: Terminal palette and muted colour

- Decision: the drawn terminal uses the Tokyo Night colours read from the README screenshot (ground `#1a1b26`, text `#a9b1d6`), with muted text `#8a93bd`.
- Why: ghist draws with ANSI colours, so a visitor's own terminal theme decides the real look; the figure matches the screenshot. The original muted `#565f89` measured 2.76 against the ground, below 4.5; `#8a93bd` measures 5.69.
- Breaks if ignored: reverting to `#565f89` puts dates and line numbers below readable contrast.
- Rejected: leaving `#565f89` because it matches the screenshot - dates are information, not decoration.

### RV-D6: Typeface pairing

- Decision: Bricolage Grotesque with JetBrains Mono, both from Google Fonts.
- Why: the owner chose the pairing from a sketch; a three-candidate specimen render was not run.
- Breaks if ignored: a font that fails to load leaves a fallback face in every image; `render.py` prints a WARN for that.

### RV-D7: Dark hero is lossy

- Decision: `assets/hero-dark.webp` is re-encoded lossy at quality 94 (283 KB); the other three images stay lossless.
- Why: lossless measured 318 to 322 KB over the 300 KB budget, driven by the 1.6 degree tilt of the terminal text; the exact command is in the header comment of `assets/source/figures.html`.
- Breaks if ignored: a plain re-render restores the lossless file at about 320 KB and breaks the budget.
- Rejected: raising `--max-kb` - it silences the warning without shrinking the file. Removing the tilt - the tilt is part of the signature.

### RV-D8: Where sources and images live

- Decision: sources in `assets/source/`, rendered images in `assets/`.
- Why: CI runs only `make check` and `make build` (`.github/workflows/ci.yml`), and nothing lints or publishes `assets/`.
- Breaks if ignored: sources placed where a linter or docs site covers them get linted or published by accident.

### RV-D9: What the README keeps

- Decision: every earlier fact stays; the key table moves into `<details>` titled "All keys"; installation and run commands stay visible; the original screenshot stays.
- Why: the screenshot is a real product image, and commands must stay copyable.
- Breaks if ignored: a command hidden in `<details>` is a visitor's first failure.

### RV-D10: Workflow figure branches

- Decision: draw only branches confirmed in `internal/tui/model.go`: `Enter` or `d` from History to Diff, `Enter` or `f` from Diff to File, `Esc` back one screen, `Esc` from History to the picker only when ghist started without a file, and on any screen `p` older, `n` newer, `o` first PR or MR, `q` quit.
- Why: in `key()`, `esc` from History quits to the picker only when `pickerEnabled`; otherwise it does nothing.
- Breaks if ignored: drawing `Esc` as always returning to the picker misstates the direct-file mode.

### RV-D11: Explainer figures and their README placement

- Decision: the README carries four explainer figures beside the hero and the workflow: `renames` after the workflow, `readonly` after it, `reviews` after the PR and MR paragraph in Installation, `layers` in Development above the package list. Each has alt text that narrates it, and the neighbouring text stays.
- Why: each answers one reader question that text answers more slowly, and the alt text plus the neighbouring text are the only text form with images off.
- Breaks if ignored: a figure that replaces its text leaves a reader with images off with no explanation.
- Rejected: only some of the four, or none - the owner chose all four.

### RV-D12: Rename figure examples and caveat

- Decision: the `renames` figure uses made-up names (`draft.go`, `service.go`, commit ids `a1b2c3d` to `e5f6071`, dates in March and April 2026) and states that Git's rename detection decides which renames can be followed.
- Why: examples inside an image use obviously fictional names; `docs/plan.md` says "Git's rename heuristics define which renames can be followed".
- Breaks if ignored: made-up commits that look real read as a project record, and dropping the caveat promises that every rename is followed.

### RV-D13: Basis of the read-only figure

- Decision: the `readonly` figure lists the five Git commands found in `internal/git` (`rev-parse`, `log --follow`, `diff-tree`, `show`, `ls-tree`), the three things ghist never does, and a timeline shown up to the HEAD read once at open, with later commits and uncommitted changes not shown.
- Why: sources are `internal/git/repository.go` (`rev-parse --verify HEAD^{commit}`), `docs/plan.md` (read-only, no checkout, HEAD resolved once, working-tree comparison out of scope) and the README (no network needed to load history). None was observed running; see RV-Q3.
- Breaks if ignored: a new Git call that writes or switches a revision makes the figure false, and a new read command leaves the list incomplete.

### RV-D14: Meaning of arrows in the layers figure

- Decision: an arrow points to what a box uses. Drawn: `cmd/ghist` to `internal/tui`, `internal/history` and `internal/git`; `internal/tui` to `internal/history` and `internal/hosting`; `internal/git` to `internal/history` and to the `git` CLI. The arrow from `cmd/ghist` to `internal/hosting` is not drawn.
- Why: an import scan showed `internal/tui` importing history and hosting, `internal/git` importing history, `internal/history` and `internal/hosting` importing no internal package, and `cmd/ghist` importing all four.
- Breaks if ignored: a reversed arrow claims `internal/history` depends on the adapter and on the interface.
- Rejected: an arrow from `cmd/ghist` to `internal/hosting` - it would cross the `internal/tui` box and be unreadable; the `cmd/ghist` box text says it builds the pieces.

## Open questions

### RV-Q1: github.com rendering

Does the README render correctly on github.com in light, dark and at phone width, including `<picture>` inside `<h1>` and inside `<details>`? Not observed. Until someone looks, the hero keeps a light `<img>` fallback inside the `<picture>`.

### RV-Q2: Commit grouping and authorization

How are the changes committed: one commit or split (images and sources, then README), and who commits? The owner has not authorized a commit. Recommendation: split into two commits, run by the owner or on their explicit go-ahead.

### RV-Q3: Later commits excluded while ghist is open

Does a commit made after ghist opened stay out of the History screen in the real application? The `readonly` figure draws it (RV-D13) from `docs/plan.md` and the code, not from an observed run. Recommendation: open ghist on a scratch repository, commit once in another terminal, and confirm the list does not change.
