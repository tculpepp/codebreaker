# Codebreaker: Web App Plan (v2.0.0)

**Version:** 1.0.0 (approved) · **Updated:** 2026-11-22
**Implements:** `docs/requirements.md` v1.3.0: D19–D25 and "Web App Requirements". Everything else in the requirements (rules, scoring, modes, D18 Easy) applies unchanged.
**Companion to:** `docs/implementation-plan.md`, which covers the terminal version. Its §1 rules (heavy comments, `Go note:`s, test-first, 100% coverage where possible) apply here too.

## 0. Goal and ground rules

Build a second front end that runs in the browser as an installable Progressive Web App (PWA), so the game runs like a native app on a Chromebook: a launcher icon, its own window, and offline play.

Ground rules:
1. **The terminal version doesn't change.** `cmd/codebreaker`, `input`, `ui` and `app` keep their behavior, and their golden files must not change. If a shared package is refactored, the unchanged golden files prove the terminal still behaves the same.
2. **One set of rules.** The web version uses `code`, `game`, `solver` and `player.Computer` as they are. No rule is written twice.
3. **The logic is tested in plain Go.** Everything that decides what's on screen lives in a package with no browser code, tested with `go test` like the rest. The browser-only layer is kept thin.
4. **No new dependencies.** The Go side adds none, and the page uses plain HTML, CSS and JavaScript, with no framework, npm or bundler.

Checked before writing this plan (Go 1.27.4): `GOOS=js GOARCH=wasm go build ./internal/...` succeeds, and the `code`, `game` and `solver` tests pass when run as WebAssembly under Node (`go test -exec=$(go env GOROOT)/lib/wasm/go_js_wasm_exec`).

## 1. How it fits together

```
 browser page (web/)                          Go, compiled to WebAssembly
 ┌─────────────────────────┐   events (JSON)   ┌──────────────────────────────┐
 │ index.html, style.css   │ ────────────────▶ │ cmd/web          (js/wasm)   │
 │ render.js: draws a View │                   │   └─ internal/web/bridge     │
 │ keyboard + tap handlers │ ◀──────────────── │        (syscall/js, thin)    │
 └─────────────────────────┘   View (JSON)     │ internal/web     (pure Go)   │
   sw.js (offline cache)                       │   Session: menus + game      │
   manifest.webmanifest                        │   Board: game.Observer       │
                                               │   Tapper: game.Player        │
                                               ├──────────────────────────────┤
                                               │ game · code · solver ·       │
                                               │ player.Computer   (shared)   │
                                               └──────────────────────────────┘
```

- **Go decides, JavaScript draws.** After every change, Go sends the page a complete description of the screen (a `View`). `render.js` turns that into HTML. The page sends back small events such as "tapped Red", "Submit" or "chose menu item 2". This answers open question 1 (§9).
- **Why this split:** the decisions (what's valid, what to show, whose turn it is) stay in Go, where they're tested and commented like the rest of the project. The JavaScript only draws, so it stays short, about 200–300 lines. Building the page from Go through `syscall/js` would work too, but every DOM call becomes long-winded `js.Value` code that can't be tested without a browser.

### Dependency rules
```
cmd/web ─▶ internal/web/bridge ─▶ internal/web ─▶ game, code, solver, player
                                                 (never input, ui or app)
```
`player` imports `input` for the terminal `Human`, which pulls `golang.org/x/term` into the web build. It compiles (x/term has a stub for js), and measured in v1.2.0 it adds about 40 KB (1%), so `Computer` stays where it is.

## 2. Project structure (new files only)

```
cmd/web/
  main.go              //go:build js && wasm. Starts the bridge, then waits forever.
internal/web/          pure Go: no syscall/js, tested natively
  doc.go
  view.go              View, Screen, BoardView, PegView, MenuView: what the page draws
  event.go             Event, EventKind, ParseEvent (from JSON)
  session.go           Session: the menu state machine and game, driven by events
  board.go             Board: a game.Observer that keeps the board for the View
  tapper.go            Tapper: a game.Player whose guesses and secrets come from events
  draft.go             Draft: the 5-slot row being built (add, undo, clear, complete?)
  pacer.go             Pacer: the pause between the computer's guesses (fake in tests)
  messages.go          friendly error text for the web (see §4)
  *_test.go, testdata/*.golden.json
internal/web/bridge/
  bridge.go            //go:build js && wasm. Connects Session to the page with syscall/js.
  bridge_test.go       //go:build js && wasm. Runs under Node in CI.
web/
  index.html           the page shell, plus a "loading" screen and a no-WebAssembly message
  style.css            layout, peg colors, light/dark themes
  render.js            draws a View; sends events
  sw.js                service worker: offline cache
  manifest.webmanifest name, icons, colors, display: standalone
  icons/               192 px, 512 px and maskable PNGs, plus an SVG source
tools/serve/main.go    tiny static file server for local testing (`make serve`)
```

Build output goes to `dist/` (git-ignored): the web files, `codebreaker.wasm`, and `wasm_exec.js` copied from `$(go env GOROOT)/lib/wasm`. **`wasm_exec.js` must come from the same Go version that built the `.wasm`**, so the Makefile always copies it fresh and never commits it.

## 3. Key data structures

```go
// View is everything the page needs to draw one screen. It's sent as JSON
// after every change; the page never keeps game state of its own.
type View struct {
	Screen  Screen     `json:"screen"`            // "name", "menu", "board", "secret", "handoff", "summary", "result", "help"
	Title   string     `json:"title"`
	Lines   []string   `json:"lines,omitempty"`   // short text: line-up, coin flip, summaries
	Menu    *MenuView  `json:"menu,omitempty"`    // numbered choices, when Screen is "menu"
	Board   *BoardView `json:"board,omitempty"`   // header, scores, rows, draft
	Message string     `json:"message,omitempty"` // a friendly error, shown above the input
	Input   InputView  `json:"input"`             // which controls are active: palette, undo, submit, skip, ready, text
}

type BoardView struct {
	Header  string    `json:"header"`  // "Single game · Round 1"
	Maker   string    `json:"maker"`   // "Red Computer"
	Breaker string    `json:"breaker"`
	Scores  []string  `json:"scores,omitempty"` // match only, Blue first
	Rows    []RowView `json:"rows"`
	Draft   []PegView `json:"draft"`   // the row being built; empty slots are zero PegViews
	RowsMax int       `json:"rowsMax"`
	Easy    bool      `json:"easy"`
}

type RowView struct {
	Row      int       `json:"row"`
	Guess    []PegView `json:"guess"`
	Feedback []string  `json:"feedback"` // "exact", "partial", "none" × 5, sorted (Normal) or by position (Easy)
}

type PegView struct {
	Letter string `json:"letter"` // "R"; "" for an empty slot
	Name   string `json:"name"`   // "Red", for screen readers
}

// Event is one thing the player did on the page.
type Event struct {
	Kind   EventKind `json:"kind"`   // "color", "undo", "submit", "choose", "text", "ready", "skip", "help", "quit"
	Letter string    `json:"letter"` // for "color"
	Choice int       `json:"choice"` // for "choose", 0-based
	Text   string    `json:"text"`   // for "text" (names)
}
```

Colors travel as letters (`"R"`), and the page maps letters to CSS colors. The palette in `style.css` matches the terminal's (§3 of the implementation plan) and is chosen for contrast on both themes.

## 4. Core pieces

### `Session` (the web version of `app`'s state machine)
- The same states as the terminal: name → main menu → difficulty → opponent → second name → role → line-up → play → play again. They're rewritten for events rather than shared with `app`, because `app` is built around a `Prompter` that asks one question and waits for a typed line. The flow is the same; only the input model differs.
- `NewSession(rng *rand.Rand, pacer Pacer, show func(View)) *Session`, `Handle(ev Event)`, and `Run(ctx) error`.
- `Run` plays the game in its own goroutine, using `game.NewMatch` as `app` does. `Handle` is called from page events. It **never blocks**: it sends the event on a buffered channel and returns, because a blocked JavaScript callback would freeze the page.
- Quit (D16) cancels the round's context and shows the standings, using `game.Standings` as the terminal does.

### `Tapper` (a `game.Player`)
- `Guess(ctx, info, history)` shows the board with an empty draft, then reads events: a color adds to the draft, Undo removes the last peg, and Submit with 5 pegs makes the guess. It checks with `code.Validate` and `game.CheckRepeat` and shows a message for mistakes (a color already in the draft is simply refused). It returns on a valid guess or when `ctx` is cancelled.
- `MakeSecret(ctx, info)` works the same way on a "secret" screen. In Human vs Human, after Confirm it shows the handoff screen and waits for "I'm ready" (D21).
- This is the same pattern as the terminal's request-driven input reader: a goroutine waiting on a channel, with `select` on `ctx.Done()`.

### `Board` (a `game.Observer`)
- Like `ui.Renderer`, it remembers the round info and turns, and builds `BoardView`s. Feedback comes from `Turn.Feedback` (sorted) or `Turn.Marks` (by position, when `RoundInfo.Easy`), so Normal and Easy look the same on both front ends.

### `Pacer`
- `Wait(ctx) error` between the computer's guesses: 700 ms in the browser, instant in tests. Skip makes the remaining waits instant. An interface is used so tests never sleep.

### Friendly messages
- The terminal's wording lives in `ui.FriendlyMessage`, and `internal/web` must not import `ui`. **Refactor (v1.2.0):** move `FriendlyMessage`, `Friendly` and their tests from `ui` into a new pure package `internal/message`. `ui` calls it, so the terminal output is identical and the golden files prove it. Both front ends then explain mistakes in the same words.

### `bridge` (the only browser code in Go)
- `Start(s *web.Session)`: registers one JavaScript function, `codebreakerSend(json)`, which parses an `Event` and calls `s.Handle`. Each `View` is encoded to JSON and passed to `codebreakerRender(json)`, which `render.js` defines.
- `Go note:` topics here: `syscall/js`, `js.FuncOf` and why it must not block, `select {}` to keep `main` alive, and build constraints.

### `render.js`
- `render(view)` replaces the page body's content from the View. A full redraw each time is simple and fast enough for a board this size.
- Keyboard: letters send `color`, Backspace sends `undo`, Enter sends `submit`, `?` sends `help`.
- Heavily commented too, but kept small. It's the only JavaScript worth reading.

## 5. Screens and interaction

- **Board screen:** the header and scores at the top, then 12 row slots (played rows filled, the next one highlighted), the draft row, the palette (8 large buttons, each with its letter), and Undo and Submit. The legend (● right color, right spot · △ right color, wrong spot) is under the board, plus the "Easy mode" line on Easy.
- **Layout:** one column, about 420 px wide, centered. It fits a 1366×768 Chromebook screen with no scrolling and still works at phone width.
- **Pegs:** colored circles with the letter on them, in a color chosen for contrast against that peg. Key pegs are a filled dot (exact), an outlined triangle (partial) and a faint dot (none). Each peg has an `aria-label`, such as "Red" or "right color, right spot".
- **Computer breaking:** rows appear one at a time (`Pacer`), with Skip.
- **Human vs Human:** a "secret" screen, then a full-screen "Pass the device to Ann" card with a single "I'm ready" button. The secret is never in the DOM while the Breaker's screen is shown; the View for the Breaker simply doesn't contain it.
- **Help:** a sheet over the current screen with the rules, scoring table and controls. Close returns to the same screen.

## 6. PWA, offline and publishing

- **`manifest.webmanifest`:** name "Codebreaker" (open question 3), `display: standalone`, theme colors, and icons at 192 px, 512 px and maskable.
- **`sw.js`:** on install, caches every file in `dist/`. Fetches are cache-first. The cache name includes the build version (stamped in by `make web`), so a new release replaces the old cache, and the page offers "Update available: reload".
- **Installing on a Chromebook:** open the URL in Chrome, then Install (in the address bar). The app then appears in the launcher and opens in its own window, online or offline.
- **Publishing (open question 2):** GitHub Pages, deployed by a GitHub Actions workflow from `dist/` on each tag. This depends on the still-pending step of publishing the repository, and Pages on a free plan needs it to be public. Any static host works too: the site is just files, with `.wasm` served as `application/wasm`.
- **Local:** `make web` builds `dist/`, and `make serve` serves it at `http://localhost:8080`. Service workers are allowed on localhost, so offline mode can be tested before publishing.

## 7. Size and performance

- Measured in v1.2.0 (Go 1.27.4, `-ldflags="-s -w"`, a probe program that plays a whole match and encodes a View): **4.5 MB raw, 1.24 MB gzipped**. That's more than first estimated, but GitHub Pages compresses it, and after the first visit it loads from the cache.
- The solver is no concern: a whole round of solving takes about 0.36 ms in WebAssembly under Node, against 0.24 ms natively (`BenchmarkRound`).
- TinyGo (much smaller output) is out of scope: it doesn't support all of the standard library this code uses, and it adds a second toolchain.

## 8. Testing

| Layer | How | Coverage goal |
|---|---|---|
| `code`, `game`, `solver`, `player` | Unchanged, plus a CI job that also runs them as WebAssembly under Node | as now (100%) |
| `internal/message` | The moved `ui` tests | 100% |
| `internal/web` | Natively, with `go test`. Scripted events in, Views out. Golden JSON files (`testdata/*.golden.json`, with `-update`) for key screens. Fake `Pacer`. A full scripted match and Human vs Human game. The race detector checks that `Handle` and `Run` share state safely | 100% |
| `internal/web/bridge` | `GOOS=js GOARCH=wasm go test` under Node: events in, rendered JSON out, using a fake `codebreakerRender` | as high as practical |
| `render.js`, CSS, PWA | By hand, from a checklist kept in this plan: every screen, keyboard and touch, light and dark, offline after first load, install and launch on a Chromebook, a fresh deploy updating the cache | checklist |
| Terminal | Every existing test and golden file unchanged | as now |

CI gains a `web` job: build `dist/`, run the WebAssembly tests under Node, and fail if `.wasm` passes a size budget (6 MB raw, about a third above the 4.5 MB measured in v1.2.0). Browser automation (such as Playwright) is out of scope for v2.0.0, so it adds no npm tooling, and could follow later.

## 9. Decisions (approved 2026-11-22, recorded as D23–D25)

| # | Question | Answer |
|---|---|---|
| 1 | Go or JavaScript draws the page? | **Go sends a View, JavaScript draws it** (§1). Decisions stay in tested Go, and the JavaScript stays short. |
| 2 | Where is it published? | **GitHub Pages**, from a public repository, deployed by Actions on each tag. Until then, `make serve` locally. |
| 3 | App name and icon | **"Codebreaker"**, with an icon of five colored pegs over a ● △ row, drawn as SVG and exported to PNG. |

## 10. Milestones

Versions follow SemVer. These are new features with no breaking changes to the terminal, so they're minor releases until v2.0.0, which marks the web app's release as the project's second front end. The process is unchanged: TDD, show red then green, approval before every commit, an annotated tag per milestone.

| Version | Scope | Done when |
|---|---|---|
| ✅ v1.2.0 | **Groundwork.** Move friendly messages to `internal/message` (terminal golden files unchanged). Add `internal/web` with `View`, `Event`, `Draft` and `Board`. Measure the wasm size and the solver's speed. Add a CI job running the shared tests as WebAssembly. | `internal/web` builds Views for a scripted round at 100% coverage. Terminal unchanged. |
| ✅ v1.3.0 | **First playable in the browser:** `Session` (Single game, human breaks the computer's code), `Tapper`, `bridge`, `cmd/web`, `web/index.html`, `render.js`, `style.css`, `make web` and `make serve`. | Play a Single game against the computer in Chrome at localhost, by tapping and by keyboard. |
| v1.4.0 | **All of the game:** every menu (match, Normal/Easy, roles, coin flip), computer as Breaker with `Pacer` and Skip, round and match summaries, help, quit with standings, play again. | Everything a terminal player against the computer can do, the web player can do. |
| v1.5.0 | **Human vs Human:** names, the secret screen, the handoff card, "I'm ready" (D21). | A scripted Human vs Human match passes, and the secret never appears in a Breaker's View (tested). |
| v2.0.0 | **Installable and published:** manifest, icons, service worker with versioned cache and update prompt, accessibility pass, README section ("Play in the browser / on a Chromebook"), Pages deploy workflow, the manual checklist done on a real Chromebook. | Installed on a Chromebook from the published URL, launched from the launcher, a full match played offline. |

## 11. Risks

| Risk | Mitigation |
|---|---|
| A blocking JavaScript callback freezes the page | `Handle` only sends on a buffered channel. A test sends many events without a running game and checks that `Handle` never blocks. |
| `wasm_exec.js` doesn't match the Go version | It's copied from `GOROOT` on every build and never committed. |
| An old cached version keeps running after a release | The cache name is versioned per build, and the page shows an update prompt. Checked in the manual checklist. |
| The web and terminal versions drift apart | Shared rules, shared Easy marks (`Turn.Marks`) and shared friendly messages. Only the flow is written twice, and its tests mirror the terminal's transcripts. |
| GitHub Pages needs a public repository | Decide before v2.0.0 (§9, item 2). Any static host works as a fallback. |
| School-managed Chromebooks may block installing apps | Out of our control. The app still runs as a normal web page in Chrome. |

## 12. Out of scope for v2.0.0

- Online play between two devices (it would need a server)
- An Android or Play Store app, and Chrome Apps (retired by Google)
- Sound, animations beyond the computer's paced rows, saved high scores
- TinyGo, JavaScript frameworks, npm tooling, browser test automation

## 13. Status

**v1.2.0 (groundwork) is done.** How it differs from the sketches above:
- `internal/message` holds `For(err)` (the friendly wording, formerly `ui.FriendlyMessage`) and the `Friendly` interface. `ui.ChoiceError` now has a `Friendly` method, so `message` explains menu mistakes without importing `ui`. Every terminal golden file is unchanged.
- `code.Feedback.Marks()` gives the Normal-difficulty marks (exact, then partial, then none). The terminal's `Renderer.Feedback` and the web `Board` both use it, so the order lives in one place.
- `internal/web` has `View`, `Event`/`ParseEvent`, `Draft` and `Board`, at 100% coverage. Changes from §3:
  - Players are `SeatView`s (`name`, `peg` as "blue" or "red" for CSS, `score`) rather than preformatted strings, so the page can color them. `BoardView` has `title`, `maker`, `breaker`, `scores` (match only, Blue first), `target`, `rows`, `draft`, `maxRows` and `easy`.
  - There's a `close` event, to close help.
  - `Draft` refuses a color already in the row (with the usual duplicate message), a full row (`ErrDraftFull`, "The row is full. Submit it, or Undo to change a peg."), and an invalid color.
  - `Board` also keeps the last round's result and the final standings, for the summary screens.
  - `testdata/board_match.golden.json` is the first part of the contract with `render.js`.
- `make test-wasm` runs `code`, `game`, `solver`, `player`, `message` and `web` as WebAssembly under Node.js, and CI has a `wasm` job doing the same with Go 1.24 and the latest Go. Locally all six pass. CI still hasn't run, as the repository isn't published yet.
- Measurements are in §7: 4.5 MB raw and 1.24 MB gzipped, `x/term` costs about 1%, and the solver is fast.

**v1.3.0 (first playable in the browser) is done.** How it differs from the sketches above:
- `game.Bracket` (moved from `ui`) gives the row range for a score, for both front ends' summaries. Terminal golden files are unchanged.
- `Session`: name → line-up ("Start") → board → result with the secret revealed → "Play again" (back to the line-up). The player always breaks the computer's code; menus come in v1.4.0. Names are trimmed, default to "Player", and are cut to 20 characters. Quit shows "Game stopped before the code was cracked." without revealing the secret.
- The event queue is a mutex-guarded slice with a one-slot "doorbell" channel, so `Handle` never blocks and never drops an event. Help and Close are handled in one place (`Session.next`): taps behind the help sheet are dropped, and closing help shows the current screen again, draft included.
- `Tapper` implements both roles. `MakeSecret` (the secret screen) is tested now but first used in v1.4.0.
- `BoardView` gained `secret`, present only once a round has ended.
- `bridge` (`Render`, `Listen`) is tested under Node.js at 100% coverage with a fake `codebreakerRender`. `cmd/web` runs the session in `main`, so no `select {}` is needed.
- The page draws the board on the left and the text and controls on the right on screens at least 760 px wide. At 1366×650 (a Chromebook's viewport) all 12 rows and the palette fit without scrolling; on a phone they stack. The last name used is remembered in `localStorage`.
- `tools/serve`: a static server with `Cache-Control: no-store` (its handler is tested; `main` isn't, like `cmd/codebreaker`). `make web` builds `dist/` (4.4 MB wasm), `make serve` runs it at http://localhost:8080. `make vet`/`make lint` also check the WebAssembly-only code, and CI builds `dist/` and checks the 6 MB budget.
- Checked in headless Chrome (driven over the DevTools Protocol) at 440×900 and 1366×650: name, line-up, tapping and typing colors, Enter, the duplicate message, Undo, help and Escape, Quit, and the result screen, with no console errors. Then checked again in a real Chrome window through the Claude in Chrome extension: keyboard play, tapping, help over a half-built row, and the remembered name (now selected, so typing replaces it). Help lists one scoring range per line, so a range never wraps mid-way.

**Next: v1.4.0**, all of the game: menus, Normal/Easy, the computer as Breaker with paced rows, matches, help, quit and play again (§10).

