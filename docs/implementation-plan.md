# Codebreaker: Go Implementation Plan

**Version:** 2.0.0 · **Updated:** 2026-11-21
**Implements:** `docs/requirements.md` v1.1.0. Decision IDs (D1–D17) refer to that document's decisions log.

## 0. Requirements baseline

Every rule question is settled. The ones that shape this design most:

| Decision | Effect on the design |
|---|---|
| D2, D3 | A round is solved at 5 exact matches and ends after 12 rows. |
| D4 | Only one scoring rule: the Breaker scores by bracket. No pluggable scorers and no `-rules` flag. |
| D5, D15 | The match ends **immediately** when any player reaches 30 or more points. There is no fixed number of rounds and no "even rounds" check. |
| D6 | A coin flip (seeded RNG) picks the first Maker. Roles swap every round. |
| D8 | The game computes feedback itself. |
| D11 | Each seat has a scoring color: Blue or Red. |
| D13 | Single Game is one round, with roles picked in the menu or by coin flip. |
| D14 | A repeated guess is rejected and does not use a row. |
| D16 | No round cap. `q` quits and shows the standings. |
| D17 | CPU vs CPU exists only in tests and demos, not in the menu. |

## 1. Code as a learning tool (applies to all code)

This project doubles as a way to learn Go, so **code is heavily commented**. Comments teach Go as well as explain the game.

### Commenting rules
1. **Package docs:** every package has a `doc.go` with a package comment covering:
   - what the package does
   - where it sits in the architecture
   - which Go concepts it demonstrates (see the concept map below)
2. **Every identifier has a doc comment**, exported or not, in godoc style that starts with the name: `// Score compares a guess with the secret and...`. The comment describes behavior, inputs, outputs, and errors.
3. **`// Go note:` comments** explain a Go idiom or language feature the first time it appears in each package. Examples: why `Code` is an array and not a slice, value receivers versus pointer receivers, implicit interfaces, `errors.As`. One consistent prefix means `grep -rn "Go note:"` lists every lesson.
4. **Comments explain why, not just what.** Function bodies get comments on intent, invariants, and non-obvious steps (for example, the bit tricks in `Score`). Comments don't restate trivial lines.
5. **Tests are commented too.** Each test says what behavior it proves and why that case matters. Table-driven tests label each case with a `name` field.
6. **Code stays readable before it's clever.** Plain, idiomatic Go is preferred over brevity. Where a shortcut is used, a comment explains it and shows the longhand version.
7. **Comments must stay accurate.** A wrong comment counts as a bug. Reviews check comments along with the code.
8. **Linting enforces the basics:** `staticcheck.conf` turns on the doc-comment checks ST1000, ST1020, ST1021 and ST1022, and `make doc` runs `go doc -all` on each package.

### Concept map (what each package teaches)

| Package | Go concepts demonstrated |
|---|---|
| `code` | Arrays vs slices, named types, `iota` constants, methods, the `fmt.Stringer` interface, custom error types, `errors.As`, `math/bits`, `sync.OnceValue`, table tests, fuzz tests, benchmarks |
| `game` | Structs and encapsulation (unexported fields), pointer receivers, interfaces declared where they're used, sentinel errors (`errors.Is`), wrapping with `%w`, returning copies to protect state |
| `solver` | Slices and filtering in place, injected `*rand.Rand` (`math/rand/v2`), deterministic tests |
| `input` | `io.Reader`/`io.Writer`, `bufio.Scanner`, goroutines, channels, `select`, `context.Context` cancellation, `defer` cleanup |
| `ui` | `strings.Builder`, ANSI escape codes, build tags (`vt_windows.go`), golden-file tests, the `-update` flag |
| `player` | Composition, satisfying interfaces implicitly, checking interface satisfaction at compile time (`var _ I = (*T)(nil)`) |
| `app` | State machines, the `flag` package, `signal.NotifyContext`, keeping `main` small, exit codes |

## 2. Project structure

```
codebreaker/
├── go.mod                       # module codebreaker; go 1.24+ (math/rand/v2, sync.OnceValue, testing.B.Loop)
├── Makefile                     # test, cover, lint, build, run, doc
├── staticcheck.conf             # adds ST1000/ST1020-22 doc-comment checks
├── README.md                    # how to play + "learning Go with this repo" guide
├── .github/workflows/ci.yml     # (v1.0.0) linux/macos/windows matrix
├── cmd/codebreaker/main.go      # flags, signal ctx, app.Run, exit codes (~40 lines)
└── internal/
    ├── code/       # PURE RULES, zero deps
    │   ├── doc.go  color.go  code.go  parse.go  feedback.go  generate.go  errors.go
    ├── solver/     # computer codebreaker
    │   ├── doc.go  solver.go
    ├── game/       # STATE, no I/O
    │   ├── doc.go  round.go  match.go  scoring.go  player.go  observer.go  config.go  errors.go
    ├── player/     # implementations
    │   ├── doc.go  human.go  computer.go
    ├── input/      # INPUT HANDLING
    │   ├── doc.go  prompter.go  hidden.go
    ├── ui/         # RENDERING
    │   ├── doc.go  ansi.go  vt_windows.go  vt_other.go  render.go  menu.go  messages.go  testdata/*.golden
    └── app/        # wiring + menu state machine
        ├── doc.go  app.go  flow.go
```

Package dependencies only point one way, so there are no import cycles:

```
cmd → app → {player, ui, input, game}
player → {game, solver, input, ui, code}
game → code;  solver → code;  ui → {game, code}
```

- `game` declares the interfaces it uses (`CodeMaker`, `CodeBreaker`, `Observer`). This is standard Go practice and gets a `Go note:` comment.
- The only external dependency is `golang.org/x/term` (hidden input, terminal detection).

## 3. Key data structures

```go
// code
type Color uint8
const (Black Color = iota; White; Red; Blue; Green; Yellow; Pink; Orange; NumColors)
const Length = 5
type Code [Length]Color                      // array: comparable, usable as a map key, copied by value
type Feedback struct{ Exact, Partial int }   // ● black key peg / △ white key peg (D9)

// game
const (MaxRows = 12; TargetScore = 30)       // D3, D5: fixed by the rules
type Mode int                                // Single, Match
type PegColor int                            // Blue, Red (D11)
type FirstMaker int                         // FirstMakerCoinFlip (zero value), FirstMakerSeat0, FirstMakerSeat1
type Config struct { Mode Mode; FirstMaker FirstMaker } // zero value = full match + coin flip
type Turn struct { Row int; Guess code.Code; Feedback code.Feedback }
type RoundStatus int                         // InProgress, Solved, OutOfRows, Abandoned
type Round struct { secret code.Code; turns []Turn; status RoundStatus }
type RoundResult struct { Number, MakerIdx, BreakerIdx, Rows int; Solved bool
                          Secret code.Code; Points int }        // Points go to the Breaker
type Seat struct { Name string; Peg PegColor; Player Participant; Score int }
type Match struct { cfg Config; seats [2]Seat; makerIdx int; observer Observer
                    results []RoundResult }
type Standings struct { Seats [2]Seat; Results []RoundResult; Winner int } // -1 = none (Single mode or quit)
type RoundInfo struct { Number, MaxRows int; Mode Mode; Maker, Breaker Seat }
```

Colors live in one table: K=Black (ANSI 240, gray so it shows on dark terminals), W=255, R=196, B=33, G=40, Y=226, P=205, O=208. Scoring pegs: Blue=33, Red=196.

## 4. Core functions

### `code` (pure; build this first, test-first)

```go
func (c Color) Abbrev() rune; func (c Color) Name() string
func ColorFromRune(r rune) (Color, bool)          // case-insensitive
func New(colors ...Color) (Code, error)
func (c Code) Validate() error
func (c Code) String() string                     // "RGYBK"
func Parse(s string) (Code, error)                // "RGYBK", "r g y b k", "R,G,Y,B,K"
func Score(secret, guess Code) Feedback
func (f Feedback) Solved() bool                   // Exact == Length (D2)
func Random(r *rand.Rand) Code                    // r.Perm(8)[:5]
func All() []Code                                 // 8P5 = 6720 codes, built once with sync.OnceValue
```

`Score` is cheap and allocates nothing, because codes never repeat a color (D7):
- `exact` = number of positions where the colors match
- `common` = `bits.OnesCount8(maskA & maskB)`, where each mask has one bit per color present
- `partial` = `common - exact`

The comments explain this trick step by step and show the slower counting version for comparison.

### `game`

```go
type CodeMaker interface   { MakeSecret(ctx context.Context, info RoundInfo) (code.Code, error) }
type CodeBreaker interface { Guess(ctx context.Context, info RoundInfo, history []Turn) (code.Code, error) }
type Participant interface { CodeMaker; CodeBreaker }
type Observer interface {
    RoundStarted(RoundInfo); TurnPlayed(RoundInfo, Turn)
    RoundEnded(RoundResult, [2]Seat); MatchEnded(Standings)
}

// scoring.go: one rule (D4), a plain function rather than an interface
func Points(rows int, solved bool) int        // 1-4→12, 5-6→10, 7-8→8, 9-10→6, 11-12→4, unsolved→0
func CheckRepeat(history []Turn, g code.Code) error  // *RepeatGuessError (D14)

func NewRound(secret code.Code) (*Round, error)
func (r *Round) Submit(g code.Code) (Turn, error)   // ErrRoundOver, *RepeatGuessError
func (r *Round) Status() RoundStatus
func (r *Round) Turns() []Turn                      // copy
func (r *Round) RowsLeft() int
func (r *Round) Secret() (code.Code, bool)          // ok only after the round is over

func NewMatch(cfg Config, seats [2]Seat, rng *rand.Rand, obs Observer) (*Match, error)
func (m *Match) PlayRound(ctx context.Context) (RoundResult, error)
func (m *Match) Play(ctx context.Context) (Standings, error)
//  Single: one PlayRound. Match: PlayRound, swap roles, repeat until a score ≥ TargetScore (D5).
func (m *Match) Standings() Standings
```

- **Repeat check (D14):** human players call `CheckRepeat` in their re-prompt loop, so a repeat never uses a row. `Round.Submit` also enforces it as a safety net. The solver never repeats a guess anyway: an earlier guess that wasn't solved scores 5 exact against itself, which doesn't match the feedback it got, so the solver has already ruled it out.
- **Why a function and not an interface:** with one scoring rule, a `Scorer` interface would add nothing. A `Go note:` comment explains when an interface is worth adding.

### `solver`

```go
func New(rng *rand.Rand) *Solver
func (s *Solver) Next() (code.Code, error)              // ErrNoCandidates
func (s *Solver) Update(g code.Code, fb code.Feedback)  // keep only candidates consistent with fb
func (s *Solver) Remaining() int
```

Start from a fixed opening guess, then pick a random code that fits all feedback so far. That averages about 5–6 rows. A test confirms it solves all 6720 codes within 12 rows. Minimax is out of scope.

### `player`

```go
func NewHuman(name string, in input.Prompter, out *ui.Renderer) *Human
//  Guess: prompt → Parse → CheckRepeat → on error FriendlyMessage + re-prompt
//  MakeSecret: hidden entry ×2 → Handoff screen (HvH only)
func NewComputer(name string, rng *rand.Rand) *Computer
//  MakeSecret: code.Random; Guess: feed history[seen:] to the solver, then Next()
```

**Drawing the board:** only the renderer draws it, by reacting to `Observer` events. `Human.Guess` only prompts and shows errors. This prevents the board being drawn twice.

### `input`

```go
type Prompter interface {
    Line(ctx context.Context, prompt string) (string, error)   // ErrQuit, ErrHelp, io.EOF
    Secret(ctx context.Context, prompt string) (string, error) // term.ReadPassword if TTY
}
func NewTerminal(in *os.File, out io.Writer) *Terminal
func NewScripted(r io.Reader, w io.Writer) *Terminal        // tests
```

`Line` reads in a goroutine and also watches `ctx.Done()`, so Ctrl-C works even while it waits for input. This is a main teaching example for goroutines, channels and `select`.

### `ui`

```go
func NewRenderer(w io.Writer, o Options) *Renderer   // NoColor, ASCII, ForceTTY
func (r *Renderer) Board(info game.RoundInfo, hist []game.Turn)
func (r *Renderer) Peg(c code.Color) string; Code(code.Code) string; Feedback(code.Feedback) string
func (r *Renderer) Scoreboard(seats [2]game.Seat)    // "Blue Alice 18/30 · Red CPU 10/30"
func (r *Renderer) Menu(title string, choices []string)
func (r *Renderer) Legend(); Error(error); Handoff(next string); Clear(); CoinFlip(first game.Seat)
func (r *Renderer) RoundSummary(game.RoundResult, [2]game.Seat); MatchSummary(game.Standings)
func ParseChoice(s string, n int) (int, error)
func FriendlyMessage(err error) string
var _ game.Observer = (*Renderer)(nil)
```

### `app` and `main`

```go
type Options struct { Mode, Opponent, Role string; Seed uint64; NoColor, ASCII bool }
func Run(ctx context.Context, o Options, in *os.File, out io.Writer) error
func run(ctx context.Context, o Options, p input.Prompter, r *ui.Renderer, rng *rand.Rand) error // testable
```

- **Flags:**
  - `-mode single|match`
  - `-opponent human|computer`
  - `-role maker|breaker|random` (Single only)
  - `-seed N`
  - `-no-color`
  - `-ascii`
- **CPU vs CPU (D17):** there's no flag for it. Tests and demos build it directly through `run`.
- **Exit codes:**
  - 0: normal finish or quit
  - 130: Ctrl-C
  - 1: internal error
  - 2: bad flags

## 5. Game flow and state

```
MainMenu → SelectMode* → SelectOpponent* → Setup(names; Blue/Red; role if Single)
  → Confirm → coin flip (shown) → ┌ PlayRound: RoundStarted → MakeSecret → [Handoff HvH]
                                  │   loop Guess → CheckRepeat → Submit → TurnPlayed
                                  │   RoundEnded (reveal, rows, points, scoreboard) → Enter
                                  │ Single: stop.  Match: stop if score ≥ 30, else swap roles
                                  └
  → MatchSummary → Play again? → MainMenu | exit
(* skipped when the matching flag is given)
```

- **Menu flow:** an explicit state machine where each state has its own step function (`type state int`, `step(ctx) (state, error)`). There are no nested loops, and each transition can be tested on its own.
- **Round states:**
  - `InProgress` → `Solved` when a guess gets 5 exact matches.
  - `InProgress` → `OutOfRows` after row 12.
  - `InProgress` → `Abandoned` when a player quits.
  - Any further `Submit` after that returns `ErrRoundOver`.
- **Ending a match (D5, D15):** the check runs after every round. Only the Breaker scores, so at most one player can cross 30 in a round, and that player wins at once.
- **Quitting (D16):** `ErrQuit` sets the round to `Abandoned`, and `Standings` is shown with `Winner = -1`.
- **Who can see what:**
  - The secret exists only inside `Round`, and `Secret()` won't return it until the round is over.
  - The computer Breaker only receives `[]Turn`.
  - `game` hands out copies of its state.
- **Randomness is passed in:** every package takes a `*rand.Rand`, so `-seed` makes a whole game reproducible, including the coin flip.

## 6. Terminal UI

```
 CODEBREAKER                                    Match · Round 3
 Maker: ● Red  CPU          Breaker: ● Blue Alice
 Score  Blue Alice 18/30 · Red CPU 10/30
 ┌────┬─────────────────────┬─────────┐
 │Row │ Guess               │ Result  │
 ├────┼─────────────────────┼─────────┤
 │  1 │ ●R ●G ●Y ●B ●K      │ ●△△··   │
 │  2 │ ●W ●R ●P ●O ●G      │ ●●△··   │
 └────┴─────────────────────┴─────────┘
 ● right color & spot   △ right color, wrong spot
 K Black W White R Red B Blue G Green Y Yellow P Pink O Orange
 Row 3/12 (RGYBK, ? help, q quit) ▸
```

- **Pegs:** each is a colored ● plus its letter, so the game works without color.
- **Feedback:** always 5 characters wide. Exact matches come first as a bold ●, then partial matches as a yellow △, then a dim `·` for each empty slot.
- **End of round:** reveal the secret, then show rows used, the points earned (and the bracket that gave them), and the updated scoreboard.
- **Redrawing:**
  - In a real terminal, the screen is cleared and redrawn each turn.
  - When output is piped, output is simply appended, with no clearing and no color.
- **When color is turned off:** `-no-color`, `NO_COLOR` set, `TERM=dumb`, or output that isn't a terminal.
- **Windows:** `vt_windows.go` turns on ANSI support and falls back to plain output if that fails. `-ascii` swaps the symbols for `* ^ + - |`.
- **Human vs Human (D10):** the secret is typed hidden, twice. Then the screen and scrollback are cleared, and the next player sees "Pass the keyboard to Bob — press Enter".
- **Errors:** shown in red above the same prompt, without redrawing the board.

## 7. Error handling

| Class | Examples | Handling |
|---|---|---|
| Validation | invalid letter, wrong length, duplicate color, repeated guess, bad menu choice | Caught in the Human and app loops: show a friendly message and ask again. Never uses a row. |
| Control flow | `ErrQuit`, `io.EOF` | Passed up to `app`, which shows the standings and exits 0. |
| Cancellation | Ctrl-C | Restore the terminal, exit 130. |
| Internal invariant | `ErrNoCandidates`, `ErrRoundOver`, a repeat reaching `Submit` | Wrapped with `%w`, exit 1. These mean bugs; tests should make them impossible. |
| Flags | unknown `-mode` value, `-role` given with `-mode match` | Print usage to stderr, exit 2. |

```go
type InvalidColorError   struct{ Char rune; Pos int }   // 1-based
type LengthError         struct{ Got int }
type DuplicateColorError struct{ Color Color; First, Second int }
type RepeatGuessError    struct{ Guess code.Code; Row int }   // game package (D14)
```

- `FriendlyMessage` turns these into messages such as:
  - "Red is used twice (positions 1 and 4). Each color may appear only once."
  - "You already tried RGYBK in row 2. Try a different code."
- Errors are checked in a fixed order (letter, then length, then duplicate, then repeat), one per attempt.
- Library code never panics on user input and never calls `os.Exit`.
- After hidden input, `defer term.Restore` always turns echo back on.

## 8. Testing (test-first)

| Package | Tests |
|---|---|
| `code` | Table tests: each accepted `Parse` format, and each error with its exact fields. Hand-checked `Score` cases. Property tests: `Score` gives the same result either way round, exact + partial ≤ 5, a code scored against itself is solved, `All()` returns 6720 unique valid codes. `FuzzParse`. Benchmarks. |
| `solver` | Solves **all 6720** codes within 12 rows and logs the average. Contradictory feedback returns `ErrNoCandidates`. Same seed gives the same guesses. Never suggests a guess it already tried. `-short` tests a sample of 500. |
| `game` | `Points` table for every row 1–12 plus unsolved. Round: state changes, the boundary at row 12, the secret staying hidden, copies returned, `RepeatGuessError`. Match with scripted fake players: roles swap, the coin flip is decided by the seed, the match ends **on the round that reaches 30** (including going past 30, e.g. 24 + 12 = 36), Single mode plays exactly one round, quitting gives `Winner = -1`, player errors are passed up, ctx cancellation. |
| `player` | Human with scripted input asks again after an invalid code and after a repeated guess, and neither uses a row. Computer is deterministic and resets between rounds. |
| `input` | Trimming, quit/help commands, EOF, ctx cancel unblocking `Line`. |
| `ui` | Golden files in plain mode (board, scoreboard, round/match summary). Stripping ANSI codes from the colored output gives the plain output. `FriendlyMessage` table, including the repeat message. |
| `app` | End-to-end with a seeded RNG and scripted input: Human vs CPU in Single and in Match, Human vs Human (hidden secret, handoff), and a CPU vs CPU match that must end with someone at 30 or more. |

- **Tooling:** `go test -race -cover ./...`, `go vet`, and `staticcheck` (with the doc-comment checks).
- **Coverage goals:** at least 90% for `code`, `game` and `solver`; at least 80% overall.

## 9. Milestones

App versions follow SemVer; commits follow Conventional Commits (AGENTS.md).

| Version | Scope |
|---|---|
| ✅ v0.1.0 | Scaffold (`go.mod`, Makefile, staticcheck.conf, `.gitignore`, README stub) and the `code` package, fully commented with `Go note:`s |
| ✅ v0.2.0 | `game`: Round, `Points`, `CheckRepeat`, Match (Single + to-30), coin flip |
| ✅ v0.3.0 | `input`, plain UI, Human player, Computer as Maker. **First playable version: Single game, a human breaks the computer's code.** |
| ✅ v0.4.0 | Solver, Computer as Breaker, **full Match mode against the computer** |
| v0.5.0 | Human vs Human: hidden input and handoff |
| v1.0.0 | Colors, Windows support, `-ascii`, flags, help, README (including the "learning Go" guide and concept map), CI on 3 OSes |

## 10. Status and next step

**Done:** v0.1.0 (`code`), v0.2.0 (`game`), v0.3.0 (`input`, `ui`, `player`, `app`, `cmd/codebreaker`) and v0.4.0 (`solver`, the computer as Code Breaker, the menu, full matches) are tagged. Every `internal` package has 100% test coverage and is clean under vet and staticcheck. `cmd/codebreaker` has no tests: `main` only wires up Ctrl-C, calls `app.Run` and exits with `app.ExitCode`. Running the binary by hand confirmed exit code 0 for quit or end of input, and 130 for Ctrl-C.

**How `game` differs from the sketches above:**
- `Config.FirstMaker` is an enum type, and its zero value means a coin flip.
- `NopObserver` is the default observer when none is given.
- `ErrMatchOver` and `ErrInvalidSetup` are sentinel errors.
- `Round.Abandon` ends a round when a player quits.
- `Match` has `Seats`, `MakerIdx` and `Over` methods.

**How v0.3.0 differs from the sketches above:**
- `input`: `Prompter` has only `Line` for now; `Secret` comes with hidden input in v0.5.0. `NewTerminal` takes an `io.Reader` rather than an `*os.File`, and v0.5.0 can check for a terminal with a type assertion. `NewScripted(out, lines...)` takes lines and echoes each one after its prompt, so test output reads as a transcript.
- `ui`: `NewRenderer(w)` has no `Options` yet; they arrive with color in v1.0.0. Output is plain text added to the end of what's on screen, with no clearing. The Renderer keeps the current round's turns itself, because `TurnPlayed` only passes the newest turn. Score lines always list Blue first. Score brackets are worked out from `game.Points`, so the screen can't disagree with the rules. There are added `Welcome`, `Help` and `Goodbye` screens and a `ChoiceError` type for menu answers.
- `player`: `NewHuman(in, screen)` takes a small `Screen` interface (`Error`, `Help`) rather than `*ui.Renderer`, which makes tests simpler. The name lives in `game.Seat`, so `NewHuman` doesn't take one. `Human.MakeSecret` reads the code in plain sight until v0.5.0. `NewComputer(rng)` doesn't take a name either, and `Computer.Guess` returns `ErrCannotBreak` until the solver arrives.
- `app`: `Run(ctx, in, out, seed)` and `ExitCode(err)`. The flow is simple: welcome, ask for a name, play a Single game, then ask "Play again?". The menu state machine waits until v0.4.0, when choosing a mode and opponent makes it worth having.

**How v0.4.0 differs from the sketches above:**
- `solver`: works as sketched. The fixed opening is RGYBK and is unexported. The test of all 6720 codes found an average of 5.37 rows and a worst case of 8, well within 12.
- `player.Computer`: starts a fresh solver whenever `history` is empty (a new round), or shorter than the history it has already seen, then passes it only the turns it hasn't seen.
- `app`: the menu is a state machine: name → main menu (Match, Single, Quit) → role (Single only: break, make, coin flip) → play → play again. Before play, `ui.Renderer.Setup` shows the line-up and the coin-flip result, and a "Press Enter to start" prompt is the confirmation. In a match, a coin flip always decides who makes the first code (D6).
- CPU vs CPU (D17) is tested in `player` (`TestComputerVsComputer`): 30 seeded matches, each ending with exactly one player at 30 or more, and the same seed replays the same match.
- Known limitation: when the computer is breaking, the board is drawn again after every one of its guesses, so its rows scroll past quickly. Clearing and redrawing the screen in v1.0.0 fixes this.

**Next: v0.5.0, Human vs Human.** Build these test-first:
1. `input.Prompter.Secret`: hidden input via `golang.org/x/term` when the input is a terminal (found with a type assertion on `NewTerminal`'s reader), and plain line input otherwise, so scripted tests still work. Restore the terminal with `defer`.
2. `player.Human.MakeSecret`: in Human vs Human, the secret is typed hidden twice and must match, then `ui.Renderer.Handoff` shows "Pass the keyboard to Bob — press Enter". Against the computer, the secret is still typed plainly.
3. `ui`: `Handoff` and `Clear`. Clearing works only in a real terminal; when output is piped, a separator line is printed instead.
4. `app`: a main-menu choice for the opponent (computer or human) and a second name prompt. Assign Blue and Red to the two players.
