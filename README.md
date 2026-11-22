# Codebreaker

The Code Breaker board game, written in Go, for the terminal and the browser. Two players take turns: one makes a secret code of 5 different colors, the other tries to break it in 12 guesses or fewer. The breaker scores more for solving in fewer rows, and the first player to reach 30 points wins.

Play against the computer or against another person on the same keyboard or device. The terminal version works in any terminal on Linux, macOS and Windows; the browser version installs as an app on a Chromebook (or any computer with Chrome or Edge) and plays offline.

## Play in the browser

**https://tculpepp.github.io/codebreaker/**

Tap the colors (or type their letters), then Submit (or press Enter). Everything runs on your device: there are no accounts, and nothing is sent anywhere.

**Install it on a Chromebook:** open the link in Chrome, then choose **Install** in the address bar (or ⋮ menu → Cast, save and share → Install page as app). Codebreaker then has its own icon in the launcher and its own window, and works without an internet connection. When a new version is published, the game offers **Reload** to switch to it.

To try a local build, `make serve` builds `dist/` and serves it at http://localhost:8080.

## Play in the terminal

Install it with Go 1.24 or later:

```sh
go install github.com/tculpepp/codebreaker/cmd/codebreaker@latest
codebreaker -h               # list the flags
```

Or from a clone of this repository:

```sh
make run                     # build and play
make build                   # build bin/codebreaker
```

## How to play

1. Choose a **Match** (rounds until someone reaches 30 points) or a **Single game** (one round), the difficulty (**Normal** or **Easy**), then your opponent: the computer or another person.
2. A coin flip decides who makes the first code. In a match, the roles swap after every round.
3. The **Code Maker** picks 5 different colors from 8. Against another person, the code is typed hidden, twice, and the screen is cleared before the keyboard is handed over.
4. The **Code Breaker** guesses. After each guess the game shows:
   - `●` for each peg of the right color in the right spot
   - `△` for each peg of the right color in the wrong spot

   The symbols are always listed in that order, so they don't say *which* pegs they mean. On the **Easy** difficulty they do: each symbol sits in the same spot as the peg it scores, so `· ● · · △` means the 2nd peg is right and the 5th is the right color in the wrong spot.
5. The Breaker scores by how many rows they needed. The Maker scores nothing.

| Solved in row | Points |
|---|---|
| 1–4 | 12 |
| 5–6 | 10 |
| 7–8 | 8 |
| 9–10 | 6 |
| 11–12 | 4 |
| Not solved | 0 |

**Typing a guess:** use the color letters `K` Black, `W` White, `R` Red, `B` Blue, `G` Green, `Y` Yellow, `P` Pink, `O` Orange, in any case: `RGYBK`, `r g y b k` or `R,G,Y,B,K`. A mistake, or a guess you've already tried this round, is explained and never costs a row.

**At any prompt:** type `?` for help or `q` to quit (the scores so far are shown). Ctrl-C also stops the game cleanly.

The full rules, and the reasons behind each decision, are in [docs/requirements.md](docs/requirements.md). The game follows the rules printed in the Code Breaker box, which aren't included here because they're the game maker's text.

## Flags

Each flag answers one of the menu questions, which is then skipped. Anything you leave out is asked as usual.

| Flag | Values | Effect |
|---|---|---|
| `-mode` | `match`, `single` | The kind of game |
| `-opponent` | `computer`, `human` | Who you play against |
| `-role` | `breaker`, `maker`, `random` | Your role in a Single game (implies `-mode single`) |
| `-seed` | a number | Fixes every random choice, so the same seed and the same answers replay the same game |
| `-no-color` | | Turns colors off (so does setting `NO_COLOR`) |
| `-ascii` | | Draws with ASCII characters only (`* ^ .` for feedback, `+ - \|` for the board) |
| `-difficulty` | `normal`, `easy` | Easy shows each feedback symbol in the same spot as the guess peg it scores |

For example, `codebreaker -mode single -opponent computer -role breaker` goes straight to breaking the computer's code.

Exit codes: 0 for a normal finish or quitting, 2 for bad flags, 130 after Ctrl-C, 1 for anything unexpected.

## Development

```sh
make test         # all tests, with the race detector
make test-short   # skips the slow test that solves all 6720 codes
make test-wasm    # the web version's packages, as WebAssembly under Node.js
make web          # build the browser version into dist/
make serve        # build it and serve it at http://localhost:8080
make cover        # coverage report (coverage.html)
make lint         # go vet + staticcheck, including doc-comment checks
make doc          # every package's documentation, as `go doc` shows it
make help         # list all targets
```

The screens are tested against **golden files** (`internal/ui/testdata`, `internal/app/testdata`), which hold the exact expected output. After a deliberate change to what the game prints, regenerate them and review the difference before committing:

```sh
go test ./internal/ui ./internal/app -update
git diff internal/*/testdata
```

The browser version is published to GitHub Pages by `.github/workflows/pages.yml` whenever a `v*` tag is pushed.

CI (`.github/workflows/ci.yml`) runs the tests on Linux, macOS and Windows with Go 1.24 and the latest Go, runs the web version's packages as WebAssembly, and checks formatting and staticcheck.

### Project layout

```
cmd/codebreaker/  terminal main: flags, Ctrl-C, exit code (kept tiny)
cmd/web/          browser main, compiled to WebAssembly (kept tiny)
internal/
  code/        the rules of a code: colors, parsing, scoring a guess (no dependencies)
  game/        rounds, scoring, matches; no input or output at all
  solver/      the computer Code Breaker
  player/      Human and Computer players
  message/     friendly wording for mistakes, shared by both front ends
  input/       terminal: reading answers, hidden input, Ctrl-C
  ui/          terminal: drawing the board, colors, menus, messages
  app/         terminal: the menus and wiring that make it a program
  web/         browser: screens as Views, events, the game session
  web/bridge/  browser: connects the session to the page (WebAssembly only)
web/           the page: index.html, style.css, render.js
tools/serve/   a small local web server for trying the browser version
docs/          requirements, implementation plan, web plan
```

Dependencies only point one way (`app` → `player`, `ui`, `input`, `game` → `code`), so the rules can be tested without a terminal, and the whole game can be played by a test script. The browser side does the same (`bridge` → `web` → `game`, `player`, `message` → `code`), and neither front end imports the other.

## Learning Go with this repo

This project is also a way to learn Go, so the code is heavily commented on purpose:

- Every package has a `doc.go` that says what the package does, where it fits, and which Go concepts it shows.
- Every function, type and constant has a doc comment.
- Each Go idiom is explained the first time it appears in a package, in a comment starting with `Go note:`.
- The tests say what they prove, and each table-driven case has a name.

To list every lesson:

```sh
grep -rn "Go note:" internal/ cmd/
```

**A suggested reading order**, from the simplest package to the most involved:

1. `internal/code`: named types, arrays, `iota`, methods, custom errors. Start with `color.go`, then `code.go`, `parse.go`, `feedback.go`.
2. `internal/game`: structs, pointer receivers, interfaces, sentinel errors. `round.go`, then `match.go`.
3. `internal/solver`: a short, complete algorithm with a slice filtered in place.
4. `internal/player`: how one type satisfies an interface defined somewhere else.
5. `internal/input`: goroutines, channels and `select`, the heart of Ctrl-C support.
6. `internal/ui`: building output, escape codes, golden-file tests.
7. `internal/app` and `cmd/codebreaker`: a state machine, flags, and a tiny `main`.

### Concept map

| Package | Go concepts shown |
|---|---|
| `code` | Arrays vs slices, named types, `iota` constants, methods, the `fmt.Stringer` interface, custom error types, `errors.As`, `math/bits`, `sync.OnceValue`, table tests, fuzz tests, benchmarks |
| `game` | Structs and encapsulation (unexported fields), pointer receivers, interfaces declared where they're used, interface embedding, sentinel errors (`errors.Is`), wrapping with `%w`, returning copies to protect state, `context.Context`, test fakes |
| `solver` | Filtering a slice in place, an injected `*rand.Rand` (`math/rand/v2`), deterministic tests of random behavior, `testing.Short`, `b.Loop` benchmarks |
| `player` | Satisfying interfaces implicitly, compile-time interface checks (`var _ I = (*T)(nil)`), small interfaces, composition, closures, method values, empty-struct error types |
| `input` | `io.Reader`/`io.Writer`, `bufio.Scanner`, goroutines, channels, `select`, `context.Context` cancellation, `sync.Once`, closing channels, type assertions, the nil-interface trap, `defer` for cleanup |
| `ui` | `strings.Builder`, ANSI escape codes, `regexp`, build tags (`vt_windows.go`), `errors.As` with an interface, golden-file tests and the `-update` flag, `t.Setenv` |
| `app` | A state machine, `flag.FlagSet`, keeping `main` small, unexported `run` for testing, exit codes, wrapping an interface by embedding it |
| `cmd/codebreaker` | `signal.NotifyContext`, why `os.Exit` skips `defer` |
| `message` | `errors.As` with concrete and interface targets, extracting a package so two others can share it |
| `web` | Struct tags and `encoding/json`, `omitempty`, pointer fields for optional parts, string enums, golden JSON files, a mutex-guarded queue with a "doorbell" channel, `select` with `default`, functions as values |
| `web/bridge`, `cmd/web` | `syscall/js`, `js.FuncOf` callbacks that must not block, build constraints, WebAssembly |
| `tools/serve` | `net/http`, `http.FileServer`, wrapping a handler with `http.HandlerFunc`, `httptest` |

## Documents

- [Requirements](docs/requirements.md): the game rules and the decisions behind them
- [Implementation plan](docs/implementation-plan.md): architecture, milestones, testing, and how the code ended up differing from the plan
- [Web app plan](docs/web-plan.md): the browser and Chromebook version, how it's built, and how it was checked

## License

[MIT](LICENSE)
